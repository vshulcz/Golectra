package grpcpublisher

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/vshulcz/Golectra/internal/application/metrics"
	"github.com/vshulcz/Golectra/internal/domain"
	grpcserver "github.com/vshulcz/Golectra/internal/infra/grpcserver"
	memrepo "github.com/vshulcz/Golectra/internal/infra/repository/memory"
	pb "github.com/vshulcz/Golectra/internal/proto/metrics"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

const testBufSize = 1024 * 1024

func TestNewClient_EmptyAddress(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("expected error for empty address")
	}
}

func TestClientClose_NilSafe(t *testing.T) {
	var c *Client
	if err := c.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
}

func TestClientSendBatch_Metadata(t *testing.T) {
	repo := memrepo.New()
	svc := metrics.New(repo, nil, nil)

	var gotIP string
	interceptor := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		vals := md.Get("x-real-ip")
		if len(vals) > 0 {
			gotIP = vals[0]
		}
		return handler(ctx, req)
	}

	lis := bufconn.Listen(testBufSize)
	srv := grpc.NewServer(grpc.UnaryInterceptor(interceptor))
	pb.RegisterMetricsServer(srv, grpcserver.NewMetricsServer(svc))
	go func() {
		_ = srv.Serve(lis)
	}()

	dialer := func(ctx context.Context, _ string) (net.Conn, error) {
		return lis.Dial()
	}
	client, err := New("bufnet", WithDialer(dialer))
	if err != nil {
		srv.Stop()
		t.Fatalf("New client: %v", err)
	}
	defer func() {
		_ = client.Close()
		srv.Stop()
		svc.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := client.SendBatch(ctx, []domain.Metrics{{
		ID:    "Alloc",
		MType: string(domain.Gauge),
		Value: floatPtr(3.14),
	}}); err != nil {
		t.Fatalf("SendBatch error: %v", err)
	}

	if gotIP == "" {
		t.Fatal("expected x-real-ip metadata to be set")
	}
}

func TestMetricToProto_Invalid(t *testing.T) {
	if _, ok := metricToProto(domain.Metrics{ID: "x", MType: "unknown"}); ok {
		t.Fatal("expected invalid metric")
	}
}

func TestClientSendOne_UsesDialer(t *testing.T) {
	repo := memrepo.New()
	svc := metrics.New(repo, nil, nil)
	lis := bufconn.Listen(testBufSize)
	srv := grpc.NewServer(grpc.UnaryInterceptor(grpcserver.TrustedSubnetInterceptor(nil)))
	pb.RegisterMetricsServer(srv, grpcserver.NewMetricsServer(svc))
	go func() {
		_ = srv.Serve(lis)
	}()

	dialer := func(ctx context.Context, _ string) (net.Conn, error) {
		return lis.Dial()
	}
	client, err := New("bufnet", WithDialer(dialer))
	if err != nil {
		srv.Stop()
		t.Fatalf("New client: %v", err)
	}
	defer func() {
		_ = client.Close()
		srv.Stop()
		svc.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	val := int64(5)
	if err := client.SendOne(ctx, domain.Metrics{ID: "PollCount", MType: string(domain.Counter), Delta: &val}); err != nil {
		t.Fatalf("SendOne error: %v", err)
	}
}

func TestClientSendBatch_Empty(t *testing.T) {
	c := &Client{}
	if err := c.SendBatch(context.Background(), nil); err != nil {
		t.Fatalf("expected nil error for empty batch, got %v", err)
	}
}

func TestClientSendBatch_AllInvalid(t *testing.T) {
	c := &Client{client: pb.NewMetricsClient(&fakeConn{})}
	if err := c.SendBatch(context.Background(), []domain.Metrics{{ID: "x", MType: "bad"}}); !errors.Is(err, domain.ErrInvalidType) {
		t.Fatalf("expected ErrInvalidType, got %v", err)
	}
}

func TestClientSendOne_Invalid(t *testing.T) {
	c := &Client{client: pb.NewMetricsClient(&fakeConn{})}
	if err := c.SendOne(context.Background(), domain.Metrics{ID: "x", MType: "bad"}); !errors.Is(err, domain.ErrInvalidType) {
		t.Fatalf("expected ErrInvalidType, got %v", err)
	}
}

type fakeConn struct{}

func (f *fakeConn) Invoke(context.Context, string, any, any, ...grpc.CallOption) error {
	return nil
}

func (f *fakeConn) NewStream(context.Context, *grpc.StreamDesc, string, ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, errors.New("not used")
}

func floatPtr(v float64) *float64 { return &v }

var _ = insecure.NewCredentials
