package config

import (
	"flag"
	"os"
	"strconv"
)

// ServerConfig содержит конфигурацию сервера метрик.
type ServerConfig struct {
	Address         string
	StoreInterval   int
	FileStoragePath string
	Restore         bool
	DatabaseDSN     string
	Key             string
}

// NewServerConfig парсит флаги и переменные окружения, возвращает конфигурацию сервера.
// Приоритет: env > flag > default.
func NewServerConfig() *ServerConfig {
	cfg := &ServerConfig{}

	flag.StringVar(&cfg.Address, "a", "localhost:8080", "address and port to run server")
	flag.IntVar(&cfg.StoreInterval, "i", 300, "store interval in seconds (0 = sync)")
	flag.StringVar(&cfg.FileStoragePath, "f", "/tmp/metrics-db.json", "file storage path")
	flag.BoolVar(&cfg.Restore, "r", true, "restore metrics from file on start")
	flag.StringVar(&cfg.DatabaseDSN, "d", "", "database connection string")
	flag.StringVar(&cfg.Key, "k", "", "key for HMAC-SHA256 signing")

	flag.Parse()

	if v := os.Getenv("ADDRESS"); v != "" {
		cfg.Address = v
	}
	if v := os.Getenv("STORE_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.StoreInterval = n
		}
	}
	if v := os.Getenv("FILE_STORAGE_PATH"); v != "" {
		cfg.FileStoragePath = v
	}
	if v := os.Getenv("RESTORE"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Restore = b
		}
	}
	if v := os.Getenv("DATABASE_DSN"); v != "" {
		cfg.DatabaseDSN = v
	}
	if v := os.Getenv("KEY"); v != "" {
		cfg.Key = v
	}

	return cfg
}
