package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/kerpe-l/metrics-alerting-service/internal/model"
)

// BatchSender отправляет батч метрик на сервер. Реализуется Sender;
// выделен в интерфейс для подмены отправителя в тестах Pool.
type BatchSender interface {
	Send(ctx context.Context, metrics []model.Metrics)
}

// Pool — worker pool отправки метрик с поддержкой штатного завершения:
// Shutdown перестаёт принимать новые батчи, но дожидается доставки всех
// уже принятых.
//
// Submit и Shutdown рассчитаны на вызов из одной горутины (главного цикла
// агента): батч, переданный в Submit конкурентно с Shutdown, может быть
// как доставлен, так и отклонён.
type Pool struct {
	jobs   chan []model.Metrics
	done   chan struct{}
	once   sync.Once
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

// NewPool создаёт пул из workers воркеров с очередью на queueSize батчей
// и сразу запускает их.
//
// Воркеры отправляют метрики с собственным контекстом, не зависящим от
// сигнального: получение сигнала завершения не должно обрывать запрос,
// который уже выполняется. Этот контекст отменяет только Shutdown.
func NewPool(sender BatchSender, workers, queueSize int) *Pool {
	sendCtx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		jobs:   make(chan []model.Metrics, queueSize),
		done:   make(chan struct{}),
		cancel: cancel,
	}
	for range workers {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.run(sendCtx, sender)
		}()
	}
	return p
}

// run — цикл воркера: вычитывает очередь до её закрытия в Shutdown,
// тем самым дорабатывая остаток после начала остановки.
func (p *Pool) run(ctx context.Context, sender BatchSender) {
	for metrics := range p.jobs {
		sender.Send(ctx, metrics)
	}
}

// Submit ставит батч в очередь отправки; если очередь полна — блокируется,
// пока воркер не освободит место. Возвращает false, если пул уже
// останавливается и батч не принят.
func (p *Pool) Submit(metrics []model.Metrics) bool {
	select {
	case <-p.done:
		return false
	default:
	}
	select {
	case p.jobs <- metrics:
		return true
	case <-p.done:
		return false
	}
}

// Shutdown штатно останавливает пул: новые батчи не принимаются, принятые
// дорабатываются. Блокируется до опустошения очереди либо до отмены ctx —
// тогда незавершённые отправки обрываются и возвращается ошибка ctx.
// Повторные вызовы безопасны.
func (p *Pool) Shutdown(ctx context.Context) error {
	p.once.Do(func() {
		close(p.done)
		close(p.jobs)
	})

	drained := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(drained)
	}()

	select {
	case <-drained:
		p.cancel()
		return nil
	case <-ctx.Done():
		// Обрываем зависшие отправки и дожидаемся выхода воркеров,
		// чтобы не оставлять горутины.
		p.cancel()
		<-drained
		return fmt.Errorf("pool shutdown: %w", ctx.Err())
	}
}
