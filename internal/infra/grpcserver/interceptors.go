package grpcserver

import (
	"context"
	"net"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type clientIPKey struct{}

// ClientIPFromContext extracts client IP injected by the interceptor.
func ClientIPFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(clientIPKey{}).(string); ok {
		return v
	}
	return ""
}

// TrustedSubnetInterceptor checks x-real-ip metadata against a trusted subnet.
func TrustedSubnetInterceptor(subnet *net.IPNet) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ip := extractClientIP(ctx)
		if subnet != nil {
			parsed := net.ParseIP(ip)
			if parsed == nil || !subnet.Contains(parsed) {
				return nil, status.Error(codes.PermissionDenied, "client IP not allowed")
			}
		}
		if ip != "" {
			ctx = context.WithValue(ctx, clientIPKey{}, ip)
		}
		return handler(ctx, req)
	}
}

func extractClientIP(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	for k, vals := range md {
		if strings.ToLower(k) != "x-real-ip" {
			continue
		}
		if len(vals) == 0 {
			return ""
		}
		return strings.TrimSpace(vals[0])
	}
	return ""
}
