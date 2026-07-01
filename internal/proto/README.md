# proto

Сгенерированный gRPC-контракт обмена метриками между агентом и сервером.
Источник — [`api/metrics.proto`](../../api/metrics.proto).

Сообщения генерируются в режиме **Opaque API**
([go.dev/blog/protobuf-opaque](https://go.dev/blog/protobuf-opaque)): поля
структур приватные, доступ только через билдеры/сеттеры (запись) и геттеры
(чтение). Прямого обращения к полям proto-объектов в коде быть не должно.

## Регенерация

Из корня репозитория:

```
protoc -I api \
  --go_out=. --go_opt=module=github.com/kerpe-l/metrics-alerting-service \
  --go_opt=default_api_level=API_OPAQUE \
  --go-grpc_out=. --go-grpc_opt=module=github.com/kerpe-l/metrics-alerting-service \
  api/metrics.proto
```
