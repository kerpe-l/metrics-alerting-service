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
}

// NewAgentConfig парсит флаги и переменные окружения, возвращает конфигурацию агента.
// Приоритет: env > flag > default.
func NewAgentConfig() (*AgentConfig, error) {
	cfg := &AgentConfig{}

	flag.StringVar(&cfg.Address, "a", "localhost:8080", "address and port of metrics server")
	flag.IntVar(&cfg.ReportInterval, "r", 10, "report interval in seconds")
	flag.IntVar(&cfg.PollInterval, "p", 2, "poll interval in seconds")
	flag.StringVar(&cfg.Key, "k", "", "key for HMAC-SHA256 signing")
	flag.IntVar(&cfg.RateLimit, "l", 1, "rate limit for concurrent requests")
	flag.StringVar(&cfg.CryptoKey, "crypto-key", "", "path to public key file for request encryption")

	flag.Parse()

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

	if cfg.PollInterval <= 0 {
		return nil, errors.New("POLL_INTERVAL must be greater than 0")
	}
	if cfg.ReportInterval <= 0 {
		return nil, errors.New("REPORT_INTERVAL must be greater than 0")
	}
	if cfg.RateLimit <= 0 {
		return nil, errors.New("RATE_LIMIT must be greater than 0")
	}

	return cfg, nil
}
