package audit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/kerpe-l/metrics-alerting-service/internal/logger"
)

// bufferSize — ёмкость очереди событий на одного наблюдателя. Если наблюдатель
// не успевает обрабатывать события, лишние отбрасываются (см. Publish).
// Переменная, а не константа, чтобы тесты могли уменьшить буфер.
var bufferSize = 64

// Publisher — субъект паттерна «Наблюдатель». На каждого наблюдателя
// заводится отдельная горутина-воркер и буферизированная очередь ёмкости
// bufferSize. Publish раскладывает событие по очередям неблокирующе: если
// очередь наблюдателя переполнена, событие для него отбрасывается с записью
// ошибки в лог и не задерживает остальных. Ошибки наблюдателей логируются и
// не прерывают доставку.
type Publisher struct {
	observers []Observer
	channels  []chan Event

	ctx    context.Context
	cancel context.CancelFunc

	wg sync.WaitGroup

	mu     sync.Mutex
	closed bool
}

// NewPublisher создаёт Publisher и запускает воркер на каждого наблюдателя.
// Набор наблюдателей фиксируется при создании и далее не меняется, поэтому
// доступ к нему не требует синхронизации.
func NewPublisher(observers ...Observer) *Publisher {
	ctx, cancel := context.WithCancel(context.Background())
	p := &Publisher{
		observers: observers,
		channels:  make([]chan Event, len(observers)),
		ctx:       ctx,
		cancel:    cancel,
	}
	p.wg.Add(len(observers))
	for i, o := range observers {
		ch := make(chan Event, bufferSize)
		p.channels[i] = ch
		go p.worker(o, ch)
	}
	return p
}

// worker последовательно обрабатывает события одного наблюдателя до закрытия
// его очереди, после чего дочитывает оставшиеся буферизированные события.
func (p *Publisher) worker(o Observer, ch <-chan Event) {
	defer p.wg.Done()
	for ev := range ch {
		if err := o.Notify(p.ctx, ev); err != nil {
			logger.Log.Error("аудит: ошибка наблюдателя: " + err.Error())
		}
	}
}

// Publish неблокирующе раскладывает событие по очередям наблюдателей.
// Если очередь наблюдателя переполнена, событие для него отбрасывается.
func (p *Publisher) Publish(ev Event) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	for i, ch := range p.channels {
		select {
		case ch <- ev:
		default:
			logger.Log.Error(fmt.Sprintf("аудит: очередь наблюдателя #%d переполнена, событие отброшено", i))
		}
	}
}

// Close переводит Publisher в закрытое состояние, дожидается обработки
// буферизированных событий и закрывает наблюдателей, реализующих io.Closer.
// Если ctx истекает раньше, чем воркеры завершатся, обработка прерывается и
// возвращается ctx.Err(); наблюдатели закрываются в любом случае. Ошибки
// закрытия наблюдателей объединяются через errors.Join и не мешают друг другу.
func (p *Publisher) Close(ctx context.Context) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	for _, ch := range p.channels {
		close(ch)
	}
	p.mu.Unlock()

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	var waitErr error
	select {
	case <-done:
	case <-ctx.Done():
		waitErr = ctx.Err()
	}
	// Отменяем внутренний контекст: разблокирует наблюдателей, висящих в
	// Notify, чтобы воркеры гарантированно завершились перед закрытием.
	p.cancel()
	<-done

	return errors.Join(waitErr, p.closeObservers())
}

// closeObservers закрывает каждого наблюдателя, реализующего io.Closer.
// Ошибки собираются и объединяются, чтобы сбой одного не отменял остальных.
func (p *Publisher) closeObservers() error {
	var errs []error
	for _, o := range p.observers {
		if c, ok := o.(io.Closer); ok {
			if err := c.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}
