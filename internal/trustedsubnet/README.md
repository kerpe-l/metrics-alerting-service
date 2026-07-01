# internal/trustedsubnet

Ограничивает приём метрик агентами из доверенной подсети (CIDR) по IP, который
агент передаёт сам. Обслуживает оба транспорта:

- **HTTP** — `Middleware(subnet *net.IPNet)`, IP из заголовка `X-Real-IP`.
  Нарушитель → `403 Forbidden`.
- **gRPC** — `UnaryInterceptor(subnet *net.IPNet)`, IP из метаданных `x-real-ip`
  (gRPC хранит ключи в нижнем регистре). Нарушитель → `codes.PermissionDenied`.

Пустой, невалидный или не входящий в подсеть IP отклоняется. Общая проверка
(`allowed`) — одна на оба транспорта.
