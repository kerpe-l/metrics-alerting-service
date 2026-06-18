package trustedsubnet

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// withRealIP строит входящий контекст с метаданными x-real-ip.
func withRealIP(ip string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(metadataKey, ip))
}

func TestUnaryInterceptor(t *testing.T) {
	tests := []struct {
		name       string
		subnet     *net.IPNet
		ctx        context.Context
		wantCode   codes.Code
		wantCalled bool
	}{
		{
			name:       "nil-подсеть пропускает без проверки",
			subnet:     nil,
			ctx:        context.Background(),
			wantCode:   codes.OK,
			wantCalled: true,
		},
		{
			name:       "IP в подсети — пропуск",
			subnet:     mustCIDR(t, "10.0.0.0/8"),
			ctx:        withRealIP("10.1.2.3"),
			wantCode:   codes.OK,
			wantCalled: true,
		},
		{
			name:       "IP вне подсети — PermissionDenied",
			subnet:     mustCIDR(t, "10.0.0.0/8"),
			ctx:        withRealIP("192.168.0.1"),
			wantCode:   codes.PermissionDenied,
			wantCalled: false,
		},
		{
			name:       "пустое значение — PermissionDenied",
			subnet:     mustCIDR(t, "10.0.0.0/8"),
			ctx:        withRealIP(""),
			wantCode:   codes.PermissionDenied,
			wantCalled: false,
		},
		{
			name:       "битый IP — PermissionDenied",
			subnet:     mustCIDR(t, "10.0.0.0/8"),
			ctx:        withRealIP("not-an-ip"),
			wantCode:   codes.PermissionDenied,
			wantCalled: false,
		},
		{
			name:       "нет метаданных — PermissionDenied",
			subnet:     mustCIDR(t, "10.0.0.0/8"),
			ctx:        context.Background(),
			wantCode:   codes.PermissionDenied,
			wantCalled: false,
		},
		{
			name:       "метаданные без ключа x-real-ip — PermissionDenied",
			subnet:     mustCIDR(t, "10.0.0.0/8"),
			ctx:        metadata.NewIncomingContext(context.Background(), metadata.Pairs("other", "x")),
			wantCode:   codes.PermissionDenied,
			wantCalled: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var called bool
			handler := func(_ context.Context, _ any) (any, error) {
				called = true
				return "ok", nil
			}

			_, err := UnaryInterceptor(tc.subnet)(tc.ctx, nil, &grpc.UnaryServerInfo{}, handler)

			if status.Code(err) != tc.wantCode {
				t.Errorf("код = %s, хотим %s", status.Code(err), tc.wantCode)
			}
			if called != tc.wantCalled {
				t.Errorf("вызов handler = %v, хотим %v", called, tc.wantCalled)
			}
		})
	}
}
