package config

import (
	"errors"
	"flag"
	"os"
	"strconv"
)

// AgentConfig содержит конфигурацию агента сбора метрик.
type AgentConfig struct {
	// Address — адрес и порт сервера метрик.
	Address string
	// ReportInterval — интервал отправки метрик на сервер, сек.
	ReportInterval int
	// PollInterval — интервал опроса метрик, сек.
	PollInterval int
	// Key — ключ для подписи HMAC-SHA256 (пусто = подпись отключена).
	Key string
	// RateLimit — максимум одновременных запросов к серверу.
	RateLimit int
	// CryptoKey — путь к файлу с публичным ключом (пусто = шифрование отключено).
	CryptoKey string
	// ShutdownTimeout — сколько ждать доставки очереди метрик при завершении, сек.
	ShutdownTimeout int
}

// agentFileConfig — представление JSON-файла конфигурации агента. Поля-указатели,
// чтобы отличать отсутствующий ключ (nil) от заданного пустого/нулевого значения.
type agentFileConfig struct {
	Address        *string `json:"address"`
	ReportInterval *string `json:"report_interval"`
	PollInterval   *string `json:"poll_interval"`
	CryptoKey       *string `json:"crypto_key"`
	Key             *string `json:"key"`
	RateLimit       *int    `json:"rate_limit"`
	ShutdownTimeout *string `json:"shutdown_timeout"`
}

// applyTo накладывает значения из файла на cfg, перекрывая только те поля, чьи флаги
// не были заданы явно (set). Так файл важнее дефолта, но слабее флага и env.
func (fc *agentFileConfig) applyTo(cfg *AgentConfig, set map[string]bool) error {
	if fc.Address != nil && !set["a"] {
		cfg.Address = *fc.Address
	}
	if fc.ReportInterval != nil && !set["r"] {
		n, err := parseDurationSeconds(*fc.ReportInterval)
		if err != nil {
			return err
		}
		cfg.ReportInterval = n
	}
	if fc.PollInterval != nil && !set["p"] {
		n, err := parseDurationSeconds(*fc.PollInterval)
		if err != nil {
			return err
		}
		cfg.PollInterval = n
	}
	if fc.CryptoKey != nil && !set["crypto-key"] {
		cfg.CryptoKey = *fc.CryptoKey
	}
	if fc.Key != nil && !set["k"] {
		cfg.Key = *fc.Key
	}
	if fc.RateLimit != nil && !set["l"] {
		cfg.RateLimit = *fc.RateLimit
	}
	if fc.ShutdownTimeout != nil && !set["shutdown-timeout"] {
		n, err := parseDurationSeconds(*fc.ShutdownTimeout)
		if err != nil {
			return err
		}
		cfg.ShutdownTimeout = n
	}
	return nil
}

// NewAgentConfig парсит флаги, JSON-файл и переменные окружения, возвращает
// конфигурацию агента. Приоритет: env > flag > config-file > default.
func NewAgentConfig() (*AgentConfig, error) {
	return parseAgentConfig(os.Args[1:])
}

// parseAgentConfig разбирает конфигурацию агента из args (без имени программы),
// env и конфиг-файла. Выделен из NewAgentConfig ради тестируемости.
func parseAgentConfig(args []string) (*AgentConfig, error) {
	cfg := &AgentConfig{}
	// ExitOnError сохраняет прежнее CLI-поведение глобального flag.Parse:
	// -h и опечатка во флаге печатают usage и завершают процесс без panic.
	fs := flag.NewFlagSet("agent", flag.ExitOnError)

	var configPath string
	fs.StringVar(&configPath, "c", "", "path to JSON config file")
	fs.StringVar(&configPath, "config", "", "path to JSON config file (alias for -c)")

	fs.StringVar(&cfg.Address, "a", "localhost:8080", "address and port of metrics server")
	fs.IntVar(&cfg.ReportInterval, "r", 10, "report interval in seconds")
	fs.IntVar(&cfg.PollInterval, "p", 2, "poll interval in seconds")
	fs.StringVar(&cfg.Key, "k", "", "key for HMAC-SHA256 signing")
	fs.IntVar(&cfg.RateLimit, "l", 1, "rate limit for concurrent requests")
	fs.StringVar(&cfg.CryptoKey, "crypto-key", "", "path to public key file for request encryption")
	fs.IntVar(&cfg.ShutdownTimeout, "shutdown-timeout", 5, "graceful shutdown timeout in seconds")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	set := setFlags(fs)

	var fc agentFileConfig
	if err := loadConfigFile(resolveConfigPath(configPath), &fc); err != nil {
		return nil, err
	}
	if err := fc.applyTo(cfg, set); err != nil {
		return nil, err
	}

	if v, ok := os.LookupEnv("ADDRESS"); ok {
		cfg.Address = v
	}
	if v, ok := os.LookupEnv("REPORT_INTERVAL"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.ReportInterval = n
		}
	}
	if v, ok := os.LookupEnv("POLL_INTERVAL"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.PollInterval = n
		}
	}
	if v, ok := os.LookupEnv("KEY"); ok {
		cfg.Key = v
	}
	if v, ok := os.LookupEnv("RATE_LIMIT"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.RateLimit = n
		}
	}
	if v, ok := os.LookupEnv("CRYPTO_KEY"); ok {
		cfg.CryptoKey = v
	}
	if v, ok := os.LookupEnv("SHUTDOWN_TIMEOUT"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.ShutdownTimeout = n
		}
	}

	if cfg.PollInterval <= 0 {
		return nil, errors.New("POLL_INTERVAL must be greater than 0")
	}
	if cfg.ReportInterval <= 0 {
		return nil, errors.New("REPORT_INTERVAL must be greater than 0")
	}
	if cfg.RateLimit <= 0 {
		return nil, errors.New("RATE_LIMIT must be greater than 0")
	}
	if cfg.ShutdownTimeout <= 0 {
		return nil, errors.New("SHUTDOWN_TIMEOUT must be greater than 0")
	}

	return cfg, nil
}
