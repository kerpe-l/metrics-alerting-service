package config

import (
	"flag"
	"os"
	"strconv"
)

// AgentConfig содержит конфигурацию агента сбора метрик.
type AgentConfig struct {
	Address        string
	ReportInterval int
	PollInterval   int
	Key            string
	RateLimit      int
}

// NewAgentConfig парсит флаги и переменные окружения, возвращает конфигурацию агента.
// Приоритет: env > flag > default.
func NewAgentConfig() *AgentConfig {
	cfg := &AgentConfig{}

	flag.StringVar(&cfg.Address, "a", "localhost:8080", "address and port of metrics server")
	flag.IntVar(&cfg.ReportInterval, "r", 10, "report interval in seconds")
	flag.IntVar(&cfg.PollInterval, "p", 2, "poll interval in seconds")
	flag.StringVar(&cfg.Key, "k", "", "key for HMAC-SHA256 signing")
	flag.IntVar(&cfg.RateLimit, "l", 1, "rate limit for concurrent requests")

	flag.Parse()

	if v := os.Getenv("ADDRESS"); v != "" {
		cfg.Address = v
	}
	if v := os.Getenv("REPORT_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.ReportInterval = n
		}
	}
	if v := os.Getenv("POLL_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.PollInterval = n
		}
	}
	if v := os.Getenv("KEY"); v != "" {
		cfg.Key = v
	}
	if v := os.Getenv("RATE_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.RateLimit = n
		}
	}

	return cfg
}
