# proto

Сгенерированный gRPC-контракт обмена метриками между агентом и сервером.
Источник — [`api/metrics.proto`](../../api/metrics.proto).

## Регенерация

Из корня репозитория:

```
protoc -I api \
  --go_out=. --go_opt=module=github.com/kerpe-l/metrics-alerting-service \
  --go-grpc_out=. --go-grpc_opt=module=github.com/kerpe-l/metrics-alerting-service \
  api/metrics.proto
```
