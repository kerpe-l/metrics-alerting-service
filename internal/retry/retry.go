package retry

import (
	"fmt"
	"time"

	"github.com/kerpe-l/metrics-alerting-service/internal/logger"
)

// delays — интервалы между повторными попытками
var delays = []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}

// Do выполняет fn и при retriable-ошибке повторяет до 3 раз с нарастающей задержкой.
func Do(fn func() error, isRetriable func(error) bool) error {
	err := fn()
	if err == nil {
		return nil
	}

	for i, delay := range delays {
		if !isRetriable(err) {
			return err
		}
		logger.Log.Warn(fmt.Sprintf("retry: attempt %d/%d failed: %v, retrying in %v", i+1, len(delays), err, delay))
		time.Sleep(delay)
		err = fn()
		if err == nil {
			return nil
		}
	}

	return err
}
