package audit

import (
	"context"
	"testing"
)

func TestWithClientIP(t *testing.T) {
	ctx := context.Background()
	ctx = WithClientIP(ctx, "127.0.0.1")
	if got := ClientIPFromContext(ctx); got != "127.0.0.1" {
		t.Fatalf("ClientIPFromContext returned %q", got)
	}

	if got := ClientIPFromContext(context.Background()); got != "" {
		t.Fatalf("expected empty ip, got %q", got)
	}
}
