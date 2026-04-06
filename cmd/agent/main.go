package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/kerpe-l/metrics-alerting-service/internal/agent"
	"github.com/kerpe-l/metrics-alerting-service/internal/config"
	"github.com/kerpe-l/metrics-alerting-service/internal/logger"
	"github.com/kerpe-l/metrics-alerting-service/internal/model"
)

func main() {
	cfg, err := config.NewAgentConfig()
	if err != nil {
		panic(err)
	}

	if err := logger.Initialize("info"); err != nil {
		panic(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	reportDuration := time.Duration(cfg.ReportInterval) * time.Second
	pollDuration := time.Duration(cfg.PollInterval) * time.Second

	serverAddr := fmt.Sprintf("http://%s", cfg.Address)

	collector := agent.NewCollector()
	sender := agent.NewSender(serverAddr, cfg.Key)

	logger.Log.Info("agent started",
		zap.Duration("poll", pollDuration),
		zap.Duration("report", reportDuration),
		zap.String("server", serverAddr),
		zap.Int("rateLimit", cfg.RateLimit),
	)

	// Канал для передачи метрик воркерам.
	jobs := make(chan []model.Metrics, cfg.RateLimit)

	// Worker pool: ограничиваем количество одновременных исходящих запросов.
	for i := 0; i < cfg.RateLimit; i++ {
		go func() {
			for metrics := range jobs {
				sender.Send(ctx, metrics)
			}
		}()
	}

	// Горутина сбора runtime-метрик.
	go func() {
		pollTicker := time.NewTicker(pollDuration)
		defer pollTicker.Stop()
		for {
			select {
			case <-pollTicker.C:
				collector.Collect()
				logger.Log.Info("Метрики собраны")
			case <-ctx.Done():
				return
			}
		}
	}()

	// Горутина сбора дополнительных метрик (gopsutil).
	go func() {
		pollTicker := time.NewTicker(pollDuration)
		defer pollTicker.Stop()
		for {
			select {
			case <-pollTicker.C:
				collector.CollectExtra()
				logger.Log.Info("Дополнительные метрики собраны")
			case <-ctx.Done():
				return
			}
		}
	}()

	// Горутина отправки метрик.
	reportTicker := time.NewTicker(reportDuration)
	defer reportTicker.Stop()
	for {
		select {
		case <-reportTicker.C:
			logger.Log.Info("Отправка метрик...")
			jobs <- collector.Metrics()
		case <-ctx.Done():
			logger.Log.Info("Агент остановлен")
			return
		}
	}
}
