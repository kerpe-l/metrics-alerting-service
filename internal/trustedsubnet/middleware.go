// Package trustedsubnet содержит middleware, ограничивающий доступ к серверу
// агентами из доверенной подсети (CIDR) по заголовку X-Real-IP.
package trustedsubnet

import (
	"net"
	"net/http"
)

// headerName — заголовок, в котором агент передаёт свой IP-адрес.
const headerName = "X-Real-IP"

// Middleware пропускает запрос дальше, только если IP из заголовка X-Real-IP
// входит в подсеть subnet. При subnet == nil проверка отключена (pass-through).
// Пустой, невалидный или не входящий в подсеть IP даёт 403 Forbidden.
func Middleware(subnet *net.IPNet) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if subnet == nil {
				next.ServeHTTP(w, r)
				return
			}

			ip := net.ParseIP(r.Header.Get(headerName))
			if ip == nil || !subnet.Contains(ip) {
				http.Error(w, "untrusted IP", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
