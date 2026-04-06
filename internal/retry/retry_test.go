package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDo(t *testing.T) {
	// Ускоряем тесты — убираем реальные задержки
	origDelays := delays
	delays = []time.Duration{0, 0, 0}
	t.Cleanup(func() { delays = origDelays })

	errRetriable := errors.New("retriable")
	errFatal := errors.New("fatal")
	alwaysRetriable := func(error) bool { return true }
	neverRetriable := func(error) bool { return false }

	tests := []struct {
		name        string
		fn          func() error
		isRetriable func(error) bool
		wantErr     error
		wantCalls   int
	}{
		{
			name:        "успех с первой попытки",
			fn:          func() error { return nil },
			isRetriable: alwaysRetriable,
			wantErr:     nil,
			wantCalls:   1,
		},
		{
			name: "успех после второй попытки",
			fn: func() func() error {
				call := 0
				return func() error {
					call++
					if call < 2 {
						return errRetriable
					}
					return nil
				}
			}(),
			isRetriable: alwaysRetriable,
			wantErr:     nil,
			wantCalls:   2,
		},
		{
			name: "успех после третьей попытки",
			fn: func() func() error {
				call := 0
				return func() error {
					call++
					if call < 3 {
						return errRetriable
					}
					return nil
				}
			}(),
			isRetriable: alwaysRetriable,
			wantErr:     nil,
			wantCalls:   3,
		},
		{
			name:        "non-retriable ошибка — без повторов",
			fn:          func() error { return errFatal },
			isRetriable: neverRetriable,
			wantErr:     errFatal,
			wantCalls:   1,
		},
		{
			name:        "все попытки провалились",
			fn:          func() error { return errRetriable },
			isRetriable: alwaysRetriable,
			wantErr:     errRetriable,
			wantCalls:   4, // 1 начальная + 3 повтора
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			wrappedFn := func() error {
				calls++
				return tt.fn()
			}

			err := Do(context.Background(), wrappedFn, tt.isRetriable)

			assert.Equal(t, tt.wantCalls, calls)
			assert.Equal(t, tt.wantErr, err)
		})
	}
}
