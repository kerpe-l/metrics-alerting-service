package pool_test

import (
	"testing"

	"github.com/kerpe-l/metrics-alerting-service/internal/pool"
)

// counter — тестовый тип с методом Reset(), который фиксирует число
// вызовов сброса, чтобы проверить поведение пула.
type counter struct {
	val    int
	resets int
}

func (c *counter) Reset() {
	c.val = 0
	c.resets++
}

func TestPool(t *testing.T) {
	t.Run("Put сбрасывает объект перед возвратом в пул", func(t *testing.T) {
		p := pool.New(func() *counter { return &counter{} })

		c := &counter{val: 42}
		p.Put(c)

		if c.val != 0 {
			t.Errorf("val после Put = %d, хотим 0", c.val)
		}
		if c.resets != 1 {
			t.Errorf("Reset вызван %d раз, хотим 1", c.resets)
		}
	})

	t.Run("Get на пустом пуле создаёт объект через newFn", func(t *testing.T) {
		p := pool.New(func() *counter { return &counter{val: 7} })

		got := p.Get()

		if got == nil {
			t.Fatal("Get вернул nil")
		}
		if got.val != 7 {
			t.Errorf("val = %d, хотим 7 (объект создан через newFn)", got.val)
		}
	})
}
