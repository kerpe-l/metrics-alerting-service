# Logger

Пакет для логирования на базе [zap](https://github.com/uber-go/zap).

## Использование

### Инициализация

Вызвать `Initialize` один раз при старте приложения:

```go
if err := logger.Initialize("info"); err != nil {
    panic(err)
}
```

Доступные уровни: `debug`, `info`, `warn`, `error`, `fatal`.

### Логирование

Можно использовать синглтон `logger.Log` в любом месте проекта:

```go
logger.Log.Info("сообщение", zap.String("key", "value"))
```

### Middleware

`RequestLogger` - middleware для HTTP-сервера. Логирует на уровне Info:

- **request** — URI, метод, время выполнения
- **response** — код статуса, размер ответа

```go
r := chi.NewRouter()
r.Use(logger.RequestLogger)
```
