package grpcserver

import (
	"context"
	"net"
	"testing"

	"github.com/vshulcz/Golectra/internal/application/metrics"
	"github.com/vshulcz/Golectra/internal/domain"
	memrepo "github.com/vshulcz/Golectra/internal/infra/repository/memory"
	pb "github.com/vshulcz/Golectra/internal/proto/metrics"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const (
	bufSize = 1024 * 1024
	okResp  = "ok"
)

func setupGRPCTestServer(t *testing.T, subnetCIDR string) (pb.MetricsClient, *memrepo.Repo, func()) {
	if t == nil {
		return nil, nil, func() {}
	}
	repo := memrepo.New()
	svc := metrics.New(repo, nil, nil)

	var subnet *net.IPNet
	if subnetCIDR != "" {
		_, parsed, err := net.ParseCIDR(subnetCIDR)
		if err != nil {
			t.Fatalf("ParseCIDR: %v", err)
		}
		subnet = parsed
	}

	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer(grpc.UnaryInterceptor(TrustedSubnetInterceptor(subnet)))
	pb.RegisterMetricsServer(srv, NewMetricsServer(svc))

	go func() {
		_ = srv.Serve(lis)
	}()

	dialer := func(ctx context.Context, _ string) (net.Conn, error) {
		return lis.Dial()
	}
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(dialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		srv.Stop()
		t.Fatalf("NewClient: %v", err)
	}

	cleanup := func() {
		_ = conn.Close()
		srv.Stop()
		svc.Close()
	}

	return pb.NewMetricsClient(conn), repo, cleanup
}

func TestUpdateMetrics_AllowsTrustedSubnet(t *testing.T) {
	client, repo, cleanup := setupGRPCTestServer(t, "10.0.0.0/24")
	defer cleanup()

	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-real-ip", "10.0.0.10"))
	_, err := client.UpdateMetrics(ctx, &pb.UpdateMetricsRequest{Metrics: []*pb.Metric{
		{Id: "Alloc", Type: pb.Metric_GAUGE, Value: 12.5},
		{Id: "PollCount", Type: pb.Metric_COUNTER, Delta: 7},
	}})
	if err != nil {
		t.Fatalf("UpdateMetrics error: %v", err)
	}

	g, err := repo.GetGauge(context.Background(), "Alloc")
	if err != nil || g != 12.5 {
		t.Fatalf("Gauge Alloc=%v err=%v", g, err)
	}
	c, err := repo.GetCounter(context.Background(), "PollCount")
	if err != nil || c != 7 {
		t.Fatalf("Counter PollCount=%v err=%v", c, err)
	}
}

func TestUpdateMetrics_DeniesUntrustedSubnet(t *testing.T) {
	client, _, cleanup := setupGRPCTestServer(t, "10.0.0.0/24")
	defer cleanup()

	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-real-ip", "192.168.1.5"))
	_, err := client.UpdateMetrics(ctx, &pb.UpdateMetricsRequest{Metrics: []*pb.Metric{{
		Id: "Alloc", Type: pb.Metric_GAUGE, Value: 1,
	}}})
	if err == nil {
		t.Fatal("expected permission denied")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected grpc status error, got %v", err)
	}
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("code=%v want %v", st.Code(), codes.PermissionDenied)
	}
}

func TestMetricFromProto_Invalid(t *testing.T) {
	_, ok := metricFromProto(&pb.Metric{Id: "x", Type: pb.Metric_MType(42)})
	if ok {
		t.Fatal("expected invalid metric")
	}
}

func TestMetricFromProto_Gauge(t *testing.T) {
	m, ok := metricFromProto(&pb.Metric{Id: "g", Type: pb.Metric_GAUGE, Value: 1.5})
	if !ok {
		t.Fatal("expected gauge to be ok")
	}
	if m.MType != string(domain.Gauge) || m.Value == nil || *m.Value != 1.5 {
		t.Fatalf("unexpected gauge mapping: %+v", m)
	}
}

func TestMetricFromProto_Counter(t *testing.T) {
	m, ok := metricFromProto(&pb.Metric{Id: "c", Type: pb.Metric_COUNTER, Delta: 3})
	if !ok {
		t.Fatal("expected counter to be ok")
	}
	if m.MType != string(domain.Counter) || m.Delta == nil || *m.Delta != 3 {
		t.Fatalf("unexpected counter mapping: %+v", m)
	}
}

func TestUpdateMetrics_InvalidRequest(t *testing.T) {
	svc := metrics.New(memrepo.New(), nil, nil)
	defer svc.Close()
	server := NewMetricsServer(svc)
	if _, err := server.UpdateMetrics(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil request")
	}
	if _, err := server.UpdateMetrics(context.Background(), &pb.UpdateMetricsRequest{}); err == nil {
		t.Fatal("expected error for empty metrics")
	}
}

func TestUpdateMetrics_InvalidMetric(t *testing.T) {
	svc := metrics.New(memrepo.New(), nil, nil)
	defer svc.Close()
	server := NewMetricsServer(svc)

	_, err := server.UpdateMetrics(context.Background(), &pb.UpdateMetricsRequest{Metrics: []*pb.Metric{{
		Id: "x", Type: pb.Metric_MType(42),
	}}})
	if err == nil {
		t.Fatal("expected error for invalid metric")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestTrustedSubnetInterceptor_NoMetadata(t *testing.T) {
	_, subnet, err := net.ParseCIDR("10.0.0.0/24")
	if err != nil {
		t.Fatalf("ParseCIDR: %v", err)
	}
	interceptor := TrustedSubnetInterceptor(subnet)
	_, err = interceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, req any) (any, error) {
		return okResp, nil
	})
	if err == nil {
		t.Fatal("expected permission denied without metadata")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", err)
	}
}

func TestTrustedSubnetInterceptor_AllowsWhenNoSubnet(t *testing.T) {
	interceptor := TrustedSubnetInterceptor(nil)
	resp, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, req any) (any, error) {
		if ip := ClientIPFromContext(ctx); ip != "" {
			return nil, status.Error(codes.Internal, "unexpected ip")
		}
		return okResp, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != okResp {
		t.Fatalf("resp=%v want ok", resp)
	}
}

func TestTrustedSubnetInterceptor_InvalidIP(t *testing.T) {
	_, subnet, err := net.ParseCIDR("10.0.0.0/24")
	if err != nil {
		t.Fatalf("ParseCIDR: %v", err)
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-real-ip", "not-an-ip"))
	interceptor := TrustedSubnetInterceptor(subnet)
	_, err = interceptor(ctx, nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, req any) (any, error) {
		return okResp, nil
	})
	if err == nil {
		t.Fatal("expected permission denied for invalid ip")
	}
}

func TestTrustedSubnetInterceptor_StoresClientIP(t *testing.T) {
	_, subnet, err := net.ParseCIDR("10.0.0.0/24")
	if err != nil {
		t.Fatalf("ParseCIDR: %v", err)
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-real-ip", "10.0.0.8"))
	interceptor := TrustedSubnetInterceptor(subnet)
	resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, req any) (any, error) {
		if ip := ClientIPFromContext(ctx); ip != "10.0.0.8" {
			return nil, status.Error(codes.Internal, "missing ip")
		}
		return okResp, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != okResp {
		t.Fatalf("resp=%v want ok", resp)
	}
}
