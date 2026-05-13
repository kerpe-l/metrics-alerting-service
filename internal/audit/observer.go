package audit

import "context"

// Observer — приёмник событий аудита.
type Observer interface {
	// Notify обрабатывает событие. Возвращённая ошибка логируется Publisher,
	// но не влияет на доставку события другим наблюдателям.
	Notify(ctx context.Context, ev Event) error
}
