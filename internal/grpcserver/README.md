# grpcserver

Реализация gRPC-сервиса `Metrics` (контракт — [internal/proto](../proto)).
Сейчас один метод — `UpdateMetrics`: приём метрик батчем и сохранение через
бизнес-логику (`service.MetricsService`).
