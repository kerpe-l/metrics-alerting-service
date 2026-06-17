// Package config парсит конфигурацию сервера и агента из флагов, переменных
// окружения и JSON-файла. Приоритет источников: env > flag > config-file > default.
package config

import (
	"errors"
	"flag"
	"os"
	"strconv"
)

// ServerConfig содержит конфигурацию сервера метрик.
type ServerConfig struct {
	// Address — адрес и порт HTTP-сервера.
	Address string
	// StoreInterval — интервал сохранения метрик в файл, сек (0 = синхронно).
	StoreInterval int
	// FileStoragePath — путь к файлу персистентности метрик.
	FileStoragePath string
	// Restore — восстанавливать метрики из файла при старте.
	Restore bool
	// DatabaseDSN — строка подключения к PostgreSQL (пусто = режим память/файл).
	DatabaseDSN string
	// Key — ключ для подписи HMAC-SHA256 (пусто = подпись отключена).
	Key string
	// AuditFile — путь к файлу аудита (пусто = файловый аудит отключён).
	AuditFile string
	// AuditURL — URL для POST событий аудита (пусто = remote-аудит отключён).
	AuditURL string
	// PprofAddr — адрес pprof debug-эндпоинта (пусто = отключён).
	PprofAddr string
	// CryptoKey — путь к файлу с приватным ключом (пусто = расшифровка отключена).
	CryptoKey string
	// ShutdownTimeout — сколько ждать завершения текущих запросов при остановке, сек.
	ShutdownTimeout int
	// TrustedSubnet — доверенная подсеть в нотации CIDR (пусто = проверка IP отключена).
	TrustedSubnet string
}

// Имена флагов сервера.
const (
	srvFlagAddress         = "a"
	srvFlagStoreInterval   = "i"
	srvFlagStoreFile       = "f"
	srvFlagRestore         = "r"
	srvFlagDatabaseDSN     = "d"
	srvFlagKey             = "k"
	srvFlagAuditFile       = "audit-file"
	srvFlagAuditURL        = "audit-url"
	srvFlagPprof           = "pprof"
	srvFlagCryptoKey       = "crypto-key"
	srvFlagShutdownTimeout = "shutdown-timeout"
	srvFlagTrustedSubnet   = "t"
)

// serverFileConfig — представление JSON-файла конфигурации сервера. Поля-указатели,
// чтобы отличать отсутствующий ключ (nil) от заданного пустого/нулевого значения.
type serverFileConfig struct {
	Address         *string `json:"address"`
	StoreInterval   *string `json:"store_interval"`
	StoreFile       *string `json:"store_file"`
	Restore         *bool   `json:"restore"`
	DatabaseDSN     *string `json:"database_dsn"`
	CryptoKey       *string `json:"crypto_key"`
	Key             *string `json:"key"`
	AuditFile       *string `json:"audit_file"`
	AuditURL        *string `json:"audit_url"`
	PprofAddr       *string `json:"pprof_addr"`
	ShutdownTimeout *string `json:"shutdown_timeout"`
	TrustedSubnet   *string `json:"trusted_subnet"`
}

// applyTo накладывает значения из файла на cfg, перекрывая только те поля, чьи флаги
// не были заданы явно (set). Так файл важнее дефолта, но слабее флага и env.
func (fc *serverFileConfig) applyTo(cfg *ServerConfig, set map[string]bool) error {
	if fc.Address != nil && !set[srvFlagAddress] {
		cfg.Address = *fc.Address
	}
	if fc.StoreInterval != nil && !set[srvFlagStoreInterval] {
		n, err := parseDurationSeconds(*fc.StoreInterval)
		if err != nil {
			return err
		}
		cfg.StoreInterval = n
	}
	if fc.StoreFile != nil && !set[srvFlagStoreFile] {
		cfg.FileStoragePath = *fc.StoreFile
	}
	if fc.Restore != nil && !set[srvFlagRestore] {
		cfg.Restore = *fc.Restore
	}
	if fc.DatabaseDSN != nil && !set[srvFlagDatabaseDSN] {
		cfg.DatabaseDSN = *fc.DatabaseDSN
	}
	if fc.CryptoKey != nil && !set[srvFlagCryptoKey] {
		cfg.CryptoKey = *fc.CryptoKey
	}
	if fc.Key != nil && !set[srvFlagKey] {
		cfg.Key = *fc.Key
	}
	if fc.AuditFile != nil && !set[srvFlagAuditFile] {
		cfg.AuditFile = *fc.AuditFile
	}
	if fc.AuditURL != nil && !set[srvFlagAuditURL] {
		cfg.AuditURL = *fc.AuditURL
	}
	if fc.PprofAddr != nil && !set[srvFlagPprof] {
		cfg.PprofAddr = *fc.PprofAddr
	}
	if fc.ShutdownTimeout != nil && !set[srvFlagShutdownTimeout] {
		n, err := parseDurationSeconds(*fc.ShutdownTimeout)
		if err != nil {
			return err
		}
		cfg.ShutdownTimeout = n
	}
	if fc.TrustedSubnet != nil && !set[srvFlagTrustedSubnet] {
		cfg.TrustedSubnet = *fc.TrustedSubnet
	}
	return nil
}

// NewServerConfig парсит флаги, JSON-файл и переменные окружения, возвращает
// конфигурацию сервера. Приоритет: env > flag > config-file > default.
func NewServerConfig() (*ServerConfig, error) {
	return parseServerConfig(os.Args[1:])
}

// parseServerConfig разбирает конфигурацию сервера из args (без имени программы),
// env и конфиг-файла. Выделен из NewServerConfig ради тестируемости.
func parseServerConfig(args []string) (*ServerConfig, error) {
	cfg := &ServerConfig{}
	// ExitOnError сохраняет прежнее CLI-поведение глобального flag.Parse:
	// -h и опечатка во флаге печатают usage и завершают процесс без panic.
	fs := flag.NewFlagSet("server", flag.ExitOnError)

	var configPath string
	fs.StringVar(&configPath, flagConfig, "", configUsageHint)
	fs.StringVar(&configPath, flagConfigLong, "", configUsageHint+" (alias for -c)")

	fs.StringVar(&cfg.Address, srvFlagAddress, "localhost:8080", "address and port to run server")
	fs.IntVar(&cfg.StoreInterval, srvFlagStoreInterval, 300, "store interval in seconds (0 = sync)")
	fs.StringVar(&cfg.FileStoragePath, srvFlagStoreFile, "/tmp/metrics-db.json", "file storage path")
	fs.BoolVar(&cfg.Restore, srvFlagRestore, true, "restore metrics from file on start")
	fs.StringVar(&cfg.DatabaseDSN, srvFlagDatabaseDSN, "", "database connection string")
	fs.StringVar(&cfg.Key, srvFlagKey, "", "key for HMAC-SHA256 signing")
	fs.StringVar(&cfg.AuditFile, srvFlagAuditFile, "", "path to audit log file (empty disables file audit)")
	fs.StringVar(&cfg.AuditURL, srvFlagAuditURL, "", "URL to POST audit events to (empty disables remote audit)")
	fs.StringVar(&cfg.PprofAddr, srvFlagPprof, "", "address for pprof debug endpoint (empty disables)")
	fs.StringVar(&cfg.CryptoKey, srvFlagCryptoKey, "", "path to private key file for request decryption")
	fs.IntVar(&cfg.ShutdownTimeout, srvFlagShutdownTimeout, 5, "graceful shutdown timeout in seconds")
	fs.StringVar(&cfg.TrustedSubnet, srvFlagTrustedSubnet, "", "trusted subnet in CIDR notation (empty disables IP check)")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	set := setFlags(fs)

	var fc serverFileConfig
	if err := loadConfigFile(resolveConfigPath(configPath), &fc); err != nil {
		return nil, err
	}
	if err := fc.applyTo(cfg, set); err != nil {
		return nil, err
	}

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
	if v, ok := os.LookupEnv("PPROF_ADDR"); ok {
		cfg.PprofAddr = v
	}
	if v, ok := os.LookupEnv("CRYPTO_KEY"); ok {
		cfg.CryptoKey = v
	}
	if v, ok := os.LookupEnv("SHUTDOWN_TIMEOUT"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.ShutdownTimeout = n
		}
	}
	if v, ok := os.LookupEnv("TRUSTED_SUBNET"); ok {
		cfg.TrustedSubnet = v
	}

	if cfg.ShutdownTimeout <= 0 {
		return nil, errors.New("SHUTDOWN_TIMEOUT must be greater than 0")
	}

	return cfg, nil
}
