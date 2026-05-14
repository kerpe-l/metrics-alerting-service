package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kerpe-l/metrics-alerting-service/internal/audit"
	"github.com/kerpe-l/metrics-alerting-service/internal/config"
	"github.com/kerpe-l/metrics-alerting-service/internal/database"
	"github.com/kerpe-l/metrics-alerting-service/internal/gzip"
	"github.com/kerpe-l/metrics-alerting-service/internal/handler"
	"github.com/kerpe-l/metrics-alerting-service/internal/hash"
	"github.com/kerpe-l/metrics-alerting-service/internal/logger"
	"github.com/kerpe-l/metrics-alerting-service/internal/repository"
	"github.com/kerpe-l/metrics-alerting-service/internal/repository/file"
	"github.com/kerpe-l/metrics-alerting-service/internal/repository/pg"
	"github.com/kerpe-l/metrics-alerting-service/internal/service"
)

func main() {
	cfg := config.NewServerConfig()

	if err := logger.Initialize("info"); err != nil {
		panic(err)
	}

	// Контекст, отменяемый по SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var st repository.Storage
	var onShutdown func() // действия при остановке (сброс метрик в файл)

	if cfg.DatabaseDSN != "" {
		// Режим БД
		if err := database.RunMigrations(cfg.DatabaseDSN); err != nil {
			logger.Log.Fatal("Ошибка миграции БД: " + err.Error())
		}

		pool, err := pgxpool.New(context.Background(), cfg.DatabaseDSN)
		if err != nil {
			logger.Log.Fatal("Не удалось подключиться к БД: " + err.Error())
		}
		defer pool.Close()

		st = pg.NewStorage(pool)
		logger.Log.Info("Хранение метрик: PostgreSQL")
	} else {
		// Режим файл/память
		storage := repository.NewMemStorage()

		// Восстанавливаем метрики из файла при старте, если задано
		if cfg.Restore && cfg.FileStoragePath != "" {
			if err := file.Load(context.Background(), cfg.FileStoragePath, storage); err != nil {
				logger.Log.Error("Не удалось загрузить метрики из файла: " + err.Error())
			} else {
				logger.Log.Info("Метрики загружены из файла " + cfg.FileStoragePath)
			}
		}

		// Запускаем периодическое сохранение, если интервал > 0
		if cfg.StoreInterval > 0 && cfg.FileStoragePath != "" {
			go func() {
				ticker := time.NewTicker(time.Duration(cfg.StoreInterval) * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						if err := file.Save(context.Background(), cfg.FileStoragePath, storage); err != nil {
							logger.Log.Error("Ошибка сохранения метрик: " + err.Error())
						}
					case <-ctx.Done():
						return
					}
				}
			}()
		}

		// Если интервал == 0, оборачиваем хранилище для синхронной записи
		st = storage
		if cfg.StoreInterval == 0 && cfg.FileStoragePath != "" {
			st = file.NewSyncStorage(storage, cfg.FileStoragePath)
		}
		logger.Log.Info("Хранение метрик: память/файл")

		// Финальный сброс метрик в файл при завершении
		if cfg.FileStoragePath != "" {
			onShutdown = func() {
				if err := file.Save(context.Background(), cfg.FileStoragePath, storage); err != nil {
					logger.Log.Error("Ошибка сохранения метрик при завершении: " + err.Error())
				} else {
					logger.Log.Info("Метрики сохранены в файл " + cfg.FileStoragePath)
				}
			}
		}
	}

	svc := service.NewMetricsService(st)

	// Аудит: создаём Publisher, если задан хотя бы один приёмник.
	auditPub, auditFileSink, err := setupAudit(cfg)
	if err != nil {
		logger.Log.Fatal("Не удалось настроить аудит: " + err.Error())
	}

	h := &handler.MetricsHandler{Service: svc, Audit: auditPub}

	r := chi.NewRouter()
	r.Use(logger.RequestLogger)
	r.Use(gzip.Middleware)
	r.Use(hash.Middleware(cfg.Key))

	r.Get("/", h.RootHandler)
	r.Post("/update/{type}/{name}/{value}", h.UpdateHandler)
	r.Get("/value/{type}/{name}", h.ValueHandler)
	r.Post("/update/", h.UpdateJSONHandler)
	r.Post("/value/", h.ValueJSONHandler)
	r.Post("/updates/", h.UpdateBatchHandler)
	r.Get("/ping", h.PingDB)

	srv := &http.Server{
		Addr:              cfg.Address,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	// Запускаем сервер в горутине
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Fatal(err.Error())
		}
	}()

	logger.Log.Info("Сервер запущен на " + cfg.Address)

	// Ждём сигнал завершения
	<-ctx.Done()
	logger.Log.Info("Получен сигнал завершения, останавливаем сервер...")

	// Даём 5 секунд на завершение текущих запросов
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error("Ошибка при остановке сервера: " + err.Error())
	}

	if onShutdown != nil {
		onShutdown()
	}

	// Дожидаемся доставки событий аудита и закрываем файл-приёмник.
	if auditPub != nil {
		if err := auditPub.Close(shutdownCtx); err != nil {
			logger.Log.Error("Ошибка ожидания аудита: " + err.Error())
		}
	}
	if auditFileSink != nil {
		if err := auditFileSink.Close(); err != nil {
			logger.Log.Error("Ошибка закрытия файла аудита: " + err.Error())
		}
	}

	logger.Log.Info("Сервер остановлен")
}

// setupAudit создаёт Publisher и регистрирует приёмники по конфигурации.
// Возвращает (nil, nil, nil), если ни один приёмник не задан.
// Файловый приёмник возвращается отдельно, чтобы закрыть его после Publisher.Close.
func setupAudit(cfg *config.ServerConfig) (*audit.Publisher, *audit.FileSink, error) {
	if cfg.AuditFile == "" && cfg.AuditURL == "" {
		return nil, nil, nil
	}

	pub := audit.NewPublisher()

	var fileSink *audit.FileSink
	if cfg.AuditFile != "" {
		fs, err := audit.NewFileSink(cfg.AuditFile)
		if err != nil {
			return nil, nil, err
		}
		fileSink = fs
		pub.Register(fs)
		logger.Log.Info("Аудит: файл " + cfg.AuditFile)
	}
	if cfg.AuditURL != "" {
		pub.Register(audit.NewHTTPSink(cfg.AuditURL))
		logger.Log.Info("Аудит: URL " + cfg.AuditURL)
	}

	return pub, fileSink, nil
}
