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

	return cfg
}
