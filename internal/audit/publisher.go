package audit

import (
	"context"
	"sync"

	"github.com/kerpe-l/metrics-alerting-service/internal/logger"
)

// Publisher — субъект паттерна «Наблюдатель». Хранит список приёмников и
// рассылает события асинхронно: каждый вызов Publish порождает по одной
// горутине на наблюдателя. Ошибки наблюдателей логируются и не прерывают
// доставку остальным.
type Publisher struct {
	observers []Observer

	ctx    context.Context
	cancel context.CancelFunc

	wg sync.WaitGroup

	mu     sync.Mutex
	closed bool
}

// NewPublisher создаёт Publisher без зарегистрированных наблюдателей.
func NewPublisher() *Publisher {
	ctx, cancel := context.WithCancel(context.Background())
	return &Publisher{ctx: ctx, cancel: cancel}
}

// Register добавляет наблюдателя.
func (p *Publisher) Register(o Observer) {
	p.observers = append(p.observers, o)
}

// Publish рассылает событие всем наблюдателям.
func (p *Publisher) Publish(ev Event) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.wg.Add(len(p.observers))
	p.mu.Unlock()

	for _, o := range p.observers {
		go func(o Observer) {
			defer p.wg.Done()
			if err := o.Notify(p.ctx, ev); err != nil {
				logger.Log.Error("аудит: ошибка наблюдателя: " + err.Error())
			}
		}(o)
	}
}

// Close переводит Publisher в закрытое состояние и дожидается завершения уже запущенных горутин рассылки.
func (p *Publisher) Close(ctx context.Context) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.cancel()
		return nil
	case <-ctx.Done():
		p.cancel()
		return ctx.Err()
	}
}
