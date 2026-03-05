package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kerpe-l/metrics-alerting-service/internal/filestorage"
	"github.com/kerpe-l/metrics-alerting-service/internal/gzip"
	"github.com/kerpe-l/metrics-alerting-service/internal/handler"
	"github.com/kerpe-l/metrics-alerting-service/internal/logger"
	"github.com/kerpe-l/metrics-alerting-service/internal/repository"
)

func main() {
	addr := flag.String("a", "localhost:8080", "address and port to run server")
	storeInterval := flag.Int("i", 300, "store interval in seconds (0 = sync)")
	fileStoragePath := flag.String("f", "/tmp/metrics-db.json", "file storage path")
	restore := flag.Bool("r", true, "restore metrics from file on start")
	databaseDSN := flag.String("d", "", "database connection string")

	flag.Parse()

	if envAddr := os.Getenv("ADDRESS"); envAddr != "" {
		*addr = envAddr
	}
	if envInterval := os.Getenv("STORE_INTERVAL"); envInterval != "" {
		if v, err := strconv.Atoi(envInterval); err == nil {
			*storeInterval = v
		}
	}
	if envPath := os.Getenv("FILE_STORAGE_PATH"); envPath != "" {
		*fileStoragePath = envPath
	}
	if envRestore := os.Getenv("RESTORE"); envRestore != "" {
		if v, err := strconv.ParseBool(envRestore); err == nil {
			*restore = v
		}
	}
	if envDSN := os.Getenv("DATABASE_DSN"); envDSN != "" {
		*databaseDSN = envDSN
	}

	if err := logger.Initialize("info"); err != nil {
		panic(err)
	}

	storage := repository.NewMemStorage()

	// Восстанавливаем метрики из файла при старте, если задано
	if *restore && *fileStoragePath != "" {
		if err := filestorage.Load(*fileStoragePath, storage); err != nil {
			logger.Log.Info("Не удалось загрузить метрики из файла: " + err.Error())
		} else {
			logger.Log.Info("Метрики загружены из файла " + *fileStoragePath)
		}
	}

	// Запускаем периодическое сохранение, если интервал > 0
	if *storeInterval > 0 && *fileStoragePath != "" {
		go func() {
			ticker := time.NewTicker(time.Duration(*storeInterval) * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				if err := filestorage.Save(*fileStoragePath, storage); err != nil {
					logger.Log.Error("Ошибка сохранения метрик: " + err.Error())
				}
			}
		}()
	}

	// Если интервал == 0, оборачиваем хранилище для синхронной записи
	var st repository.Storage = storage
	if *storeInterval == 0 && *fileStoragePath != "" {
		st = filestorage.NewSyncStorage(storage, *fileStoragePath)
	}

	var dbPool *pgxpool.Pool
	if *databaseDSN != "" {
		pool, err := pgxpool.New(context.Background(), *databaseDSN)
		if err != nil {
			logger.Log.Fatal("Не удалось подключиться к БД: " + err.Error())
		}
		defer pool.Close()
		dbPool = pool
		logger.Log.Info("Подключение к БД установлено")
	}

	h := &handler.MetricsHandler{Storage: st, DB: dbPool}

	r := chi.NewRouter()
	r.Use(logger.RequestLogger)
	r.Use(gzip.Middleware)

	r.Get("/", h.RootHandler)
	r.Post("/update/{type}/{name}/{value}", h.UpdateHandler)
	r.Get("/value/{type}/{name}", h.ValueHandler)
	r.Post("/update/", h.UpdateJSONHandler)
	r.Post("/value/", h.ValueJSONHandler)
	r.Get("/ping", h.PingDB)

	logger.Log.Info("Сервер запущен на " + *addr)

	err := http.ListenAndServe(*addr, r)
	if err != nil {
		logger.Log.Fatal(err.Error())
	}
}
