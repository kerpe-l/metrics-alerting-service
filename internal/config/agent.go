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
	// GRPCAddress — адрес gRPC-сервера (пусто = отправка по HTTP).
	GRPCAddress string
	// GRPCCACert — путь к CA-сертификату для проверки gRPC-сервера по TLS.
	GRPCCACert string
}

// Имена флагов агента.
const (
	agentFlagAddress         = "a"
	agentFlagReportInterval  = "r"
	agentFlagPollInterval    = "p"
	agentFlagKey             = "k"
	agentFlagRateLimit       = "l"
	agentFlagCryptoKey       = "crypto-key"
	agentFlagShutdownTimeout = "shutdown-timeout"
	agentFlagGRPCAddress     = "g"
	agentFlagGRPCCACert      = "grpc-ca"
)

// agentFileConfig — представление JSON-файла конфигурации агента. Поля-указатели,
// чтобы отличать отсутствующий ключ (nil) от заданного пустого/нулевого значения.
type agentFileConfig struct {
	Address         *string `json:"address"`
	ReportInterval  *string `json:"report_interval"`
	PollInterval    *string `json:"poll_interval"`
	CryptoKey       *string `json:"crypto_key"`
	Key             *string `json:"key"`
	RateLimit       *int    `json:"rate_limit"`
	ShutdownTimeout *string `json:"shutdown_timeout"`
	GRPCAddress     *string `json:"grpc_address"`
	GRPCCACert      *string `json:"grpc_ca_cert"`
}

// applyTo накладывает значения из файла на cfg, перекрывая только те поля, чьи флаги
// не были заданы явно (set). Так файл важнее дефолта, но слабее флага и env.
func (fc *agentFileConfig) applyTo(cfg *AgentConfig, set map[string]bool) error {
	if fc.Address != nil && !set[agentFlagAddress] {
		cfg.Address = *fc.Address
	}
	if fc.ReportInterval != nil && !set[agentFlagReportInterval] {
		n, err := parseDurationSeconds(*fc.ReportInterval)
		if err != nil {
			return err
		}
		cfg.ReportInterval = n
	}
	if fc.PollInterval != nil && !set[agentFlagPollInterval] {
		n, err := parseDurationSeconds(*fc.PollInterval)
		if err != nil {
			return err
		}
		cfg.PollInterval = n
	}
	if fc.CryptoKey != nil && !set[agentFlagCryptoKey] {
		cfg.CryptoKey = *fc.CryptoKey
	}
	if fc.Key != nil && !set[agentFlagKey] {
		cfg.Key = *fc.Key
	}
	if fc.RateLimit != nil && !set[agentFlagRateLimit] {
		cfg.RateLimit = *fc.RateLimit
	}
	if fc.ShutdownTimeout != nil && !set[agentFlagShutdownTimeout] {
		n, err := parseDurationSeconds(*fc.ShutdownTimeout)
		if err != nil {
			return err
		}
		cfg.ShutdownTimeout = n
	}
	if fc.GRPCAddress != nil && !set[agentFlagGRPCAddress] {
		cfg.GRPCAddress = *fc.GRPCAddress
	}
	if fc.GRPCCACert != nil && !set[agentFlagGRPCCACert] {
		cfg.GRPCCACert = *fc.GRPCCACert
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
	fs.StringVar(&configPath, flagConfig, "", configUsageHint)
	fs.StringVar(&configPath, flagConfigLong, "", configUsageHint+" (alias for -c)")

	fs.StringVar(&cfg.Address, agentFlagAddress, "localhost:8080", "address and port of metrics server")
	fs.IntVar(&cfg.ReportInterval, agentFlagReportInterval, 10, "report interval in seconds")
	fs.IntVar(&cfg.PollInterval, agentFlagPollInterval, 2, "poll interval in seconds")
	fs.StringVar(&cfg.Key, agentFlagKey, "", "key for HMAC-SHA256 signing")
	fs.IntVar(&cfg.RateLimit, agentFlagRateLimit, 1, "rate limit for concurrent requests")
	fs.StringVar(&cfg.CryptoKey, agentFlagCryptoKey, "", "path to public key file for request encryption")
	fs.IntVar(&cfg.ShutdownTimeout, agentFlagShutdownTimeout, 5, "graceful shutdown timeout in seconds")
	fs.StringVar(&cfg.GRPCAddress, agentFlagGRPCAddress, "", "gRPC server address (empty sends over HTTP)")
	fs.StringVar(&cfg.GRPCCACert, agentFlagGRPCCACert, "", "path to CA certificate for gRPC server verification")

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
	if v, ok := os.LookupEnv("GRPC_ADDRESS"); ok {
		cfg.GRPCAddress = v
	}
	if v, ok := os.LookupEnv("GRPC_CA_CERT"); ok {
		cfg.GRPCCACert = v
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
