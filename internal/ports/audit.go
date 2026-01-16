package ports

import (
	"context"

	"github.com/vshulcz/Golectra/internal/domain"
)

// AuditPublisher publishes audit events to an external sink.
type AuditPublisher interface {
	Publish(context.Context, domain.AuditEvent) error
}
