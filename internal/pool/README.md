# internal/pool

Типобезопасный контейнер для повторного использования объектов поверх
`sync.Pool`. См. godoc пакета.

## Пример

```go
p := pool.New(func() *bytes.Buffer { return &bytes.Buffer{} }) // если у *bytes.Buffer есть Reset()

b := p.Get()
defer p.Put(b) // Reset вызовется автоматически
b.WriteString("...")
```
