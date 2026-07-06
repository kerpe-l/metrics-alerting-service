package main

import (
	"context"
	"crypto/rsa"
	"errors"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/kerpe-l/metrics-alerting-service/internal/audit"
	"github.com/kerpe-l/metrics-alerting-service/internal/buildinfo"
	"github.com/kerpe-l/metrics-alerting-service/internal/config"
	"github.com/kerpe-l/metrics-alerting-service/internal/crypto"
	"github.com/kerpe-l/metrics-alerting-service/internal/database"
	"github.com/kerpe-l/metrics-alerting-service/internal/grpcserver"
	"github.com/kerpe-l/metrics-alerting-service/internal/gzip"
	"github.com/kerpe-l/metrics-alerting-service/internal/handler"
	"github.com/kerpe-l/metrics-alerting-service/internal/hash"
	"github.com/kerpe-l/metrics-alerting-service/internal/logger"
	pb "github.com/kerpe-l/metrics-alerting-service/internal/proto"
	"github.com/kerpe-l/metrics-alerting-service/internal/repository"
	"github.com/kerpe-l/metrics-alerting-service/internal/repository/file"
	"github.com/kerpe-l/metrics-alerting-service/internal/repository/pg"
	"github.com/kerpe-l/metrics-alerting-service/internal/service"
	"github.com/kerpe-l/metrics-alerting-service/internal/trustedsubnet"
)

// Сведения о сборке. Подставляются линкером через -ldflags "-X main.buildVersion=...".
var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func main() {
	// Печать сведений о сборке в stdout до инициализации логгера.
	if err := buildinfo.Fprint(os.Stdout, buildVersion, buildDate, buildCommit); err != nil {
		panic(err)
	}

	cfg, err := config.NewServerConfig()
	if err != nil {
		panic(err)
	}

	if err := logger.Initialize("info"); err != nil {
		panic(err)
	}

	// Контекст, отменяемый по SIGINT/SIGTERM/SIGQUIT
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	var st repository.Storage
	var onShutdown func() // действия при остановке (сброс метрик в файл)

	if cfg.DatabaseDSN != "" {
		// Режим БД
		if err := database.RunMigrations(cfg.DatabaseDSN); err != nil {
			logger.Log.Fatal("Ошибка миграции БД", zap.Error(err))
		}

		pool, err := pgxpool.New(context.Background(), cfg.DatabaseDSN)
		if err != nil {
			logger.Log.Fatal("Не удалось подключиться к БД", zap.Error(err))
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
				logger.Log.Error("Не удалось загрузить метрики из файла", zap.Error(err))
			} else {
				logger.Log.Info("Метрики загружены из файла", zap.String("path", cfg.FileStoragePath))
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
							logger.Log.Error("Ошибка сохранения метрик", zap.Error(err))
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
					logger.Log.Error("Ошибка сохранения метрик при завершении", zap.Error(err))
				} else {
					logger.Log.Info("Метрики сохранены в файл", zap.String("path", cfg.FileStoragePath))
				}
			}
		}
	}

	svc := service.NewMetricsService(st)

	// Аудит: создаём Publisher, если задан хотя бы один приёмник.
	auditPub, err := setupAudit(cfg)
	if err != nil {
		logger.Log.Fatal("Не удалось настроить аудит", zap.Error(err))
	}

	h := &handler.MetricsHandler{Service: svc, Audit: auditPub}

	// Загружаем приватный ключ для расшифровки запросов, если задан.
	var privKey *rsa.PrivateKey
	if cfg.CryptoKey != "" {
		privKey, err = crypto.LoadPrivateKey(cfg.CryptoKey)
		if err != nil {
			logger.Log.Fatal("не удалось загрузить приватный ключ", zap.Error(err))
		}
		logger.Log.Info("расшифровка запросов включена")
	}

	// Разбираем доверенную подсеть, если задана.
	var trustedNet *net.IPNet
	if cfg.TrustedSubnet != "" {
		_, trustedNet, err = net.ParseCIDR(cfg.TrustedSubnet)
		if err != nil {
			logger.Log.Fatal("неверный формат TRUSTED_SUBNET", zap.Error(err))
		}
		logger.Log.Info("проверка доверенной подсети включена", zap.String("subnet", cfg.TrustedSubnet))
	}

	r := chi.NewRouter()
	r.Use(logger.RequestLogger)
	// Проверка X-Real-IP до дешифровки: чужой IP отсекаем по открытому заголовку.
	// Слой регистрируем только если подсеть задана — иначе он не нужен.
	if trustedNet != nil {
		r.Use(trustedsubnet.Middleware(trustedNet))
	}
	// Расшифровка снаружи gzip: decrypt → gunzip → verify hash.
	r.Use(crypto.Middleware(privKey))
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
			logger.Log.Fatal("ошибка HTTP-сервера", zap.Error(err))
		}
	}()

	logger.Log.Info("Сервер запущен", zap.String("address", cfg.Address))

	// Опциональный gRPC-сервер на отдельном порту. Проверка доверенной подсети —
	// тем же trustedNet, что и у HTTP, но через UnaryInterceptor.
	var grpcSrv *grpc.Server
	if cfg.GRPCAddress != "" {
		// gRPC поднимаем только по TLS: без cert/key транспорт был бы открытым.
		if cfg.GRPCCertFile == "" || cfg.GRPCKeyFile == "" {
			logger.Log.Fatal("gRPC требует TLS: заданы не оба GRPC_CERT_FILE и GRPC_KEY_FILE")
		}
		creds, err := credentials.NewServerTLSFromFile(cfg.GRPCCertFile, cfg.GRPCKeyFile)
		if err != nil {
			logger.Log.Fatal("не удалось загрузить TLS для gRPC", zap.Error(err))
		}
		lis, err := net.Listen("tcp", cfg.GRPCAddress)
		if err != nil {
			logger.Log.Fatal("не удалось открыть gRPC-listener", zap.Error(err))
		}
		// Interceptor подсети добавляем только если подсеть задана.
		opts := []grpc.ServerOption{grpc.Creds(creds)}
		if trustedNet != nil {
			opts = append(opts, grpc.ChainUnaryInterceptor(trustedsubnet.UnaryInterceptor(trustedNet)))
		}
		grpcSrv = grpc.NewServer(opts...)
		pb.RegisterMetricsServer(grpcSrv, grpcserver.New(svc))
		go func() {
			if err := grpcSrv.Serve(lis); err != nil {
				logger.Log.Error("ошибка gRPC-сервера", zap.Error(err))
			}
		}()
		logger.Log.Info("gRPC сервер запущен", zap.String("address", cfg.GRPCAddress))
	}

	// Опциональный pprof debug-сервер на отдельном порту.
	// Использует DefaultServeMux, в который регистрируются хендлеры из net/http/pprof.
	var pprofSrv *http.Server
	if cfg.PprofAddr != "" {
		pprofSrv = &http.Server{
			Addr:              cfg.PprofAddr,
			Handler:           http.DefaultServeMux,
			ReadHeaderTimeout: 5 * time.Second,
		}
		go func() {
			if err := pprofSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Log.Error("ошибка pprof-сервера", zap.Error(err))
			}
		}()
		logger.Log.Info("pprof доступен", zap.String("address", cfg.PprofAddr+"/debug/pprof/"))
	}

	// Ждём сигнал завершения
	<-ctx.Done()
	logger.Log.Info("Получен сигнал завершения, останавливаем сервер...")

	// Даём текущим запросам время на завершение (по умолчанию 5 секунд)
	shutdownCtx, cancel := context.WithTimeout(context.Background(),
		time.Duration(cfg.ShutdownTimeout)*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error("Ошибка при остановке сервера", zap.Error(err))
	}
	if pprofSrv != nil {
		if err := pprofSrv.Shutdown(shutdownCtx); err != nil {
			logger.Log.Error("Ошибка при остановке pprof", zap.Error(err))
		}
	}
	if grpcSrv != nil {
		// GracefulStop дожидается текущих RPC; при истечении таймаута обрываем
		// принудительно через Stop, чтобы уложиться в ShutdownTimeout.
		stopped := make(chan struct{})
		go func() {
			grpcSrv.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-shutdownCtx.Done():
			grpcSrv.Stop()
		}
		logger.Log.Info("gRPC сервер остановлен")
	}

	if onShutdown != nil {
		onShutdown()
	}

	// Дожидаемся доставки событий аудита; Publisher сам закрывает приёмники.
	if auditPub != nil {
		if err := auditPub.Close(shutdownCtx); err != nil {
			logger.Log.Error("Ошибка завершения аудита", zap.Error(err))
		}
	}

	logger.Log.Info("Сервер остановлен")
}

// setupAudit создаёт Publisher с приёмниками по конфигурации.
// Возвращает (nil, nil), если ни один приёмник не задан. Закрытие приёмников
// (в т.ч. файлового) берёт на себя Publisher.Close.
func setupAudit(cfg *config.ServerConfig) (*audit.Publisher, error) {
	if cfg.AuditFile == "" && cfg.AuditURL == "" {
		return nil, nil
	}

	var observers []audit.Observer
	if cfg.AuditFile != "" {
		fs, err := audit.NewFileSink(cfg.AuditFile)
		if err != nil {
			return nil, err
		}
		observers = append(observers, fs)
		logger.Log.Info("Аудит: файл", zap.String("path", cfg.AuditFile))
	}
	if cfg.AuditURL != "" {
		observers = append(observers, audit.NewHTTPSink(cfg.AuditURL))
		logger.Log.Info("Аудит: URL", zap.String("url", cfg.AuditURL))
	}

	return audit.NewPublisher(observers...), nil
}
