// Package grpcserver provides gRPC transport for metrics service.
package grpcserver

import (
	"context"
	"errors"

	"github.com/vshulcz/Golectra/internal/application/metrics"
	"github.com/vshulcz/Golectra/internal/domain"
	pb "github.com/vshulcz/Golectra/internal/proto/metrics"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MetricsServer implements the gRPC Metrics service.
type MetricsServer struct {
	pb.UnimplementedMetricsServer
	svc *metrics.Service
}

// NewMetricsServer wires a metrics service into a gRPC server implementation.
func NewMetricsServer(svc *metrics.Service) *MetricsServer {
	return &MetricsServer{svc: svc}
}

// UpdateMetrics receives and stores metric batches.
func (s *MetricsServer) UpdateMetrics(ctx context.Context, req *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
	if req == nil || len(req.Metrics) == 0 {
		return nil, status.Error(codes.InvalidArgument, "metrics required")
	}
	items := make([]domain.Metrics, 0, len(req.Metrics))
	for _, m := range req.Metrics {
		item, ok := metricFromProto(m)
		if !ok {
			continue
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return nil, status.Error(codes.InvalidArgument, "metrics required")
	}

	clientIP := ClientIPFromContext(ctx)
	if _, err := s.svc.UpsertBatch(ctx, items, clientIP); err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidType):
			return nil, status.Error(codes.InvalidArgument, "invalid metric")
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return &pb.UpdateMetricsResponse{}, nil
}

func metricFromProto(m *pb.Metric) (domain.Metrics, bool) {
	if m == nil {
		return domain.Metrics{}, false
	}
	item := domain.Metrics{ID: m.Id}
	switch m.Type {
	case pb.Metric_GAUGE:
		item.MType = string(domain.Gauge)
		v := m.Value
		item.Value = &v
		return item, true
	case pb.Metric_COUNTER:
		item.MType = string(domain.Counter)
		d := m.Delta
		item.Delta = &d
		return item, true
	default:
		return domain.Metrics{}, false
	}
}
