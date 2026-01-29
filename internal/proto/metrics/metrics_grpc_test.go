package metrics

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeMetricsServer struct {
	resp *UpdateMetricsResponse
	err  error
}

func (f *fakeMetricsServer) UpdateMetrics(context.Context, *UpdateMetricsRequest) (*UpdateMetricsResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.resp != nil {
		return f.resp, nil
	}
	return &UpdateMetricsResponse{}, nil
}

func (f *fakeMetricsServer) mustEmbedUnimplementedMetricsServer() {}

type fakeRegistrar struct {
	called bool
	impl   any
}

func (f *fakeRegistrar) RegisterService(_ *grpc.ServiceDesc, impl any) {
	f.called = true
	f.impl = impl
}

type fakeConn struct {
	called bool
	method string
}

func (f *fakeConn) Invoke(_ context.Context, method string, _, reply any, _ ...grpc.CallOption) error {
	f.called = true
	f.method = method
	if resp, ok := reply.(*UpdateMetricsResponse); ok {
		*resp = UpdateMetricsResponse{}
		return nil
	}
	return status.Error(codes.Internal, "bad reply")
}

func (f *fakeConn) NewStream(context.Context, *grpc.StreamDesc, string, ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, status.Error(codes.Internal, "not used")
}

func TestRegisterMetricsServer(t *testing.T) {
	reg := &fakeRegistrar{}
	srv := &fakeMetricsServer{}
	RegisterMetricsServer(reg, srv)
	if !reg.called {
		t.Fatal("expected RegisterService to be called")
	}
	if reg.impl != srv {
		t.Fatal("expected registered impl to be srv")
	}
}

func TestUpdateMetricsHandler_NoInterceptor(t *testing.T) {
	srv := &fakeMetricsServer{resp: &UpdateMetricsResponse{}}
	dec := func(v any) error {
		req := v.(*UpdateMetricsRequest)
		req.Metrics = []*Metric{{Id: "Alloc", Type: Metric_GAUGE, Value: 1}}
		return nil
	}
	resp, err := _Metrics_UpdateMetrics_Handler(srv, context.Background(), dec, nil)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if _, ok := resp.(*UpdateMetricsResponse); !ok {
		t.Fatalf("unexpected response type: %T", resp)
	}
}

func TestUpdateMetricsHandler_WithInterceptor(t *testing.T) {
	srv := &fakeMetricsServer{}
	dec := func(v any) error { return nil }
	interceptor := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return handler(ctx, req)
	}
	if _, err := _Metrics_UpdateMetrics_Handler(srv, context.Background(), dec, interceptor); err != nil {
		t.Fatalf("handler error: %v", err)
	}
}

func TestNewMetricsClient(t *testing.T) {
	cc := &fakeConn{}
	client := NewMetricsClient(cc)
	if _, err := client.UpdateMetrics(context.Background(), &UpdateMetricsRequest{}); err != nil {
		t.Fatalf("UpdateMetrics error: %v", err)
	}
	if !cc.called || cc.method != Metrics_UpdateMetrics_FullMethodName {
		t.Fatalf("expected Invoke to be called with %s", Metrics_UpdateMetrics_FullMethodName)
	}
}
