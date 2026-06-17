package main

import (
	"context"
	"crypto/rsa"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/kerpe-l/metrics-alerting-service/internal/agent"
	"github.com/kerpe-l/metrics-alerting-service/internal/buildinfo"
	"github.com/kerpe-l/metrics-alerting-service/internal/config"
	"github.com/kerpe-l/metrics-alerting-service/internal/crypto"
	"github.com/kerpe-l/metrics-alerting-service/internal/logger"
)

// Сведения о сборке. Подставляются линкером через -ldflags "-X main.buildVersion=...".
var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func main() {
	// Печать сведений о сборке в stdout до инициализации логгера.
	if err := buildinfo.Fprint(os.Stdout, buildVersion, buildDate, buildCommit); err != nil {
		panic(err)
	}

	cfg, err := config.NewAgentConfig()
	if err != nil {
		panic(err)
	}

	if err := logger.Initialize("info"); err != nil {
		panic(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	reportDuration := time.Duration(cfg.ReportInterval) * time.Second
	pollDuration := time.Duration(cfg.PollInterval) * time.Second

	serverAddr := fmt.Sprintf("http://%s", cfg.Address)

	// Загружаем публичный ключ для шифрования запросов, если задан.
	var pubKey *rsa.PublicKey
	if cfg.CryptoKey != "" {
		pubKey, err = crypto.LoadPublicKey(cfg.CryptoKey)
		if err != nil {
			logger.Log.Fatal("не удалось загрузить публичный ключ: " + err.Error())
		}
		logger.Log.Info("шифрование запросов включено")
	}

	// Определяем свой IP для заголовка X-Real-IP. При ошибке шлём без заголовка.
	realIP, err := agent.OutboundIP(cfg.Address)
	if err != nil {
		logger.Log.Warn("не удалось определить исходящий IP: " + err.Error())
	}

	collector := agent.NewCollector()
	sender := agent.NewSender(serverAddr, cfg.Key, realIP, pubKey)

	logger.Log.Info("agent started",
		zap.Duration("poll", pollDuration),
		zap.Duration("report", reportDuration),
		zap.String("server", serverAddr),
		zap.Int("rateLimit", cfg.RateLimit),
	)

	// Worker pool: ограничивает количество одновременных исходящих запросов
	// и дорабатывает очередь при штатном завершении.
	pool := agent.NewPool(sender, cfg.RateLimit, cfg.RateLimit)

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

	// Главный цикл отправки метрик.
	reportTicker := time.NewTicker(reportDuration)
	defer reportTicker.Stop()
	for {
		select {
		case <-reportTicker.C:
			logger.Log.Info("Отправка метрик...")
			if !pool.Submit(collector.Metrics()) {
				logger.Log.Warn("пул остановлен, батч не принят")
			}
		case <-ctx.Done():
			// Штатное завершение: досылаем собранные, но ещё не
			// отправленные метрики и дожидаемся доставки очереди.
			logger.Log.Info("Получен сигнал завершения, досылаем метрики...")
			if !pool.Submit(collector.Metrics()) {
				logger.Log.Warn("пул остановлен, финальный батч не принят")
			}

			shutdownCtx, cancel := context.WithTimeout(context.Background(),
				time.Duration(cfg.ShutdownTimeout)*time.Second)
			defer cancel()
			if err := pool.Shutdown(shutdownCtx); err != nil {
				logger.Log.Error("Не все метрики доставлены при завершении: " + err.Error())
			}

			logger.Log.Info("Агент остановлен")
			return
		}
	}
}
