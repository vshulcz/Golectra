package audit

import "context"

type ctxKey string

const clientIPKey ctxKey = "audit_client_ip"

// WithClientIP stores the originating request IP inside the context for later audit fan-out.
func WithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, clientIPKey, ip)
}

// ClientIPFromContext extracts the stored client IP, returning an empty string when missing.
func ClientIPFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(clientIPKey).(string)
	return v
}
