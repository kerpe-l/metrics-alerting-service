package retry

import "time"

// delays — интервалы между повторными попытками
var delays = []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}

// Do выполняет fn и при retriable-ошибке повторяет до 3 раз с нарастающей задержкой.
func Do(fn func() error, isRetriable func(error) bool) error {
	err := fn()
	if err == nil {
		return nil
	}

	for _, delay := range delays {
		if !isRetriable(err) {
			return err
		}
		time.Sleep(delay)
		err = fn()
		if err == nil {
			return nil
		}
	}

	return err
}
