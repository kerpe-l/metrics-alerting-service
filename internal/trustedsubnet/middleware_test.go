package trustedsubnet

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mustCIDR парсит CIDR или валит тест.
func mustCIDR(t *testing.T, s string) *net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatalf("ParseCIDR(%q): %v", s, err)
	}
	return n
}

func TestMiddleware(t *testing.T) {
	tests := []struct {
		name     string
		subnet   *net.IPNet
		realIP   string // значение заголовка X-Real-IP ("" = заголовок не задан)
		wantCode int
	}{
		{
			name:     "nil-подсеть пропускает без проверки",
			subnet:   nil,
			realIP:   "",
			wantCode: http.StatusOK,
		},
		{
			name:     "IP в подсети — пропуск",
			subnet:   mustCIDR(t, "10.0.0.0/8"),
			realIP:   "10.1.2.3",
			wantCode: http.StatusOK,
		},
		{
			name:     "IP вне подсети — 403",
			subnet:   mustCIDR(t, "10.0.0.0/8"),
			realIP:   "192.168.0.1",
			wantCode: http.StatusForbidden,
		},
		{
			name:     "пустой заголовок — 403",
			subnet:   mustCIDR(t, "10.0.0.0/8"),
			realIP:   "",
			wantCode: http.StatusForbidden,
		},
		{
			name:     "битый IP — 403",
			subnet:   mustCIDR(t, "10.0.0.0/8"),
			realIP:   "not-an-ip",
			wantCode: http.StatusForbidden,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var called bool
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodPost, "/updates/", nil)
			if tc.realIP != "" {
				req.Header.Set(headerName, tc.realIP)
			}
			rec := httptest.NewRecorder()

			Middleware(tc.subnet)(next).ServeHTTP(rec, req)

			if rec.Code != tc.wantCode {
				t.Errorf("код = %d, хотим %d", rec.Code, tc.wantCode)
			}
			wantCalled := tc.wantCode == http.StatusOK
			if called != wantCalled {
				t.Errorf("вызов next = %v, хотим %v", called, wantCalled)
			}
		})
	}
}
