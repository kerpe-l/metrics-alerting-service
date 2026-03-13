package main

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kerpe-l/metrics-alerting-service/internal/config"
	"github.com/kerpe-l/metrics-alerting-service/internal/database"
	"github.com/kerpe-l/metrics-alerting-service/internal/gzip"
	"github.com/kerpe-l/metrics-alerting-service/internal/handler"
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

	var st repository.Storage

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
				logger.Log.Info("Не удалось загрузить метрики из файла: " + err.Error())
			} else {
				logger.Log.Info("Метрики загружены из файла " + cfg.FileStoragePath)
			}
		}

		// Запускаем периодическое сохранение, если интервал > 0
		if cfg.StoreInterval > 0 && cfg.FileStoragePath != "" {
			go func() {
				ticker := time.NewTicker(time.Duration(cfg.StoreInterval) * time.Second)
				defer ticker.Stop()
				for range ticker.C {
					if err := file.Save(context.Background(), cfg.FileStoragePath, storage); err != nil {
						logger.Log.Error("Ошибка сохранения метрик: " + err.Error())
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
	}

	svc := service.NewMetricsService(st)
	h := &handler.MetricsHandler{Service: svc}

	r := chi.NewRouter()
	r.Use(logger.RequestLogger)
	r.Use(gzip.Middleware)

	r.Get("/", h.RootHandler)
	r.Post("/update/{type}/{name}/{value}", h.UpdateHandler)
	r.Get("/value/{type}/{name}", h.ValueHandler)
	r.Post("/update/", h.UpdateJSONHandler)
	r.Post("/value/", h.ValueJSONHandler)
	r.Post("/updates/", h.UpdateBatchHandler)
	r.Get("/ping", h.PingDB)

	logger.Log.Info("Сервер запущен на " + cfg.Address)

	err := http.ListenAndServe(cfg.Address, r)
	if err != nil {
		logger.Log.Fatal(err.Error())
	}
}
