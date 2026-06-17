# internal/trustedsubnet

Middleware, ограничивающий приём метрик агентами из доверенной подсети (CIDR).

`Middleware(subnet *net.IPNet)` сверяет IP из заголовка `X-Real-IP` с подсетью.
При `subnet == nil` проверка отключена (прозрачный pass-through). Пустой,
невалидный или не входящий в подсеть IP отклоняется с `403 Forbidden`.
