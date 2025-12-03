package audit

import "context"

type ctxKey string

const clientIPKey ctxKey = "audit_client_ip"

func WithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, clientIPKey, ip)
}

func ClientIPFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(clientIPKey).(string)
	return v
}
