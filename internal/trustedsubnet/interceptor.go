package trustedsubnet

import (
	"context"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// metadataKey — ключ метаданных, в котором агент передаёт свой IP-адрес по gRPC.
// gRPC хранит ключи метаданных в нижнем регистре.
const metadataKey = "x-real-ip"

// UnaryInterceptor пропускает вызов дальше, только если IP из метаданных
// x-real-ip входит в подсеть subnet. При subnet == nil проверка отключена
// (pass-through). Пустой, невалидный или не входящий в подсеть IP даёт
// codes.PermissionDenied.
func UnaryInterceptor(subnet *net.IPNet) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if subnet == nil {
			return handler(ctx, req)
		}

		if !allowed(subnet, realIPFromMetadata(ctx)) {
			return nil, status.Error(codes.PermissionDenied, "untrusted IP")
		}

		return handler(ctx, req)
	}
}

// realIPFromMetadata достаёт IP агента из метаданных x-real-ip.
// Возвращает "", если метаданных или ключа нет.
func realIPFromMetadata(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get(metadataKey)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
