// Package trustedsubnet ограничивает доступ к серверу агентами из доверенной
// подсети (CIDR) по передаваемому ими IP: HTTP-middleware (заголовок X-Real-IP)
// и gRPC-UnaryInterceptor (метаданные x-real-ip).
package trustedsubnet

import (
	"net"
	"net/http"
)

// headerName — заголовок, в котором агент передаёт свой IP-адрес по HTTP.
const headerName = "X-Real-IP"

// allowed сообщает, входит ли разобранный из ipStr IP в подсеть subnet.
// Пустой или невалидный ipStr → false. subnet здесь не nil.
func allowed(subnet *net.IPNet, ipStr string) bool {
	ip := net.ParseIP(ipStr)
	return ip != nil && subnet.Contains(ip)
}

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

			if !allowed(subnet, r.Header.Get(headerName)) {
				http.Error(w, "untrusted IP", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
