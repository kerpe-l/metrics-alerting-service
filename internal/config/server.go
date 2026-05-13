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
	AuditFile       string
	AuditURL        string
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
	flag.StringVar(&cfg.AuditFile, "audit-file", "", "path to audit log file (empty disables file audit)")
	flag.StringVar(&cfg.AuditURL, "audit-url", "", "URL to POST audit events to (empty disables remote audit)")

	flag.Parse()

	if v, ok := os.LookupEnv("ADDRESS"); ok {
		cfg.Address = v
	}
	if v, ok := os.LookupEnv("STORE_INTERVAL"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.StoreInterval = n
		}
	}
	if v, ok := os.LookupEnv("FILE_STORAGE_PATH"); ok {
		cfg.FileStoragePath = v
	}
	if v, ok := os.LookupEnv("RESTORE"); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Restore = b
		}
	}
	if v, ok := os.LookupEnv("DATABASE_DSN"); ok {
		cfg.DatabaseDSN = v
	}
	if v, ok := os.LookupEnv("KEY"); ok {
		cfg.Key = v
	}
	if v, ok := os.LookupEnv("AUDIT_FILE"); ok {
		cfg.AuditFile = v
	}
	if v, ok := os.LookupEnv("AUDIT_URL"); ok {
		cfg.AuditURL = v
	}

	return cfg
}
