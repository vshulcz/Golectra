// Package grpcpublisher provides a gRPC-based metrics publisher.
package grpcpublisher

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/vshulcz/Golectra/internal/domain"
	"github.com/vshulcz/Golectra/internal/ports"
	pb "github.com/vshulcz/Golectra/internal/proto/metrics"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// Client publishes metrics to the server using gRPC.
type Client struct {
	conn   *grpc.ClientConn
	client pb.MetricsClient
	realIP string
}

var _ ports.Publisher = (*Client)(nil)

// Option configures the gRPC publisher client.
type Option func(*clientOptions)

type clientOptions struct {
	dialer func(context.Context, string) (net.Conn, error)
}

// WithDialer overrides the dialer (useful for tests).
func WithDialer(dialer func(context.Context, string) (net.Conn, error)) Option {
	return func(opts *clientOptions) {
		opts.dialer = dialer
	}
}

// New connects to the gRPC server and returns a Client instance.
func New(address string, opts ...Option) (*Client, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, errors.New("empty gRPC address")
	}

	cfg := clientOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	target := address
	dialOpts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	if cfg.dialer != nil {
		if !strings.Contains(target, "://") {
			target = "passthrough:///" + target
		}
		dialOpts = append(dialOpts, grpc.WithContextDialer(cfg.dialer))
	}

	conn, err := grpc.NewClient(target, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("grpc dial: %w", err)
	}

	return &Client{
		conn:   conn,
		client: pb.NewMetricsClient(conn),
		realIP: localIP(),
	}, nil
}

// Close releases the underlying gRPC connection.
func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// SendBatch sends a batch of metrics to the gRPC server.
func (c *Client) SendBatch(ctx context.Context, items []domain.Metrics) error {
	if len(items) == 0 {
		return nil
	}
	req := &pb.UpdateMetricsRequest{Metrics: make([]*pb.Metric, 0, len(items))}
	for _, m := range items {
		pm, ok := metricToProto(m)
		if !ok {
			continue
		}
		req.Metrics = append(req.Metrics, pm)
	}
	if len(req.Metrics) == 0 {
		return domain.ErrInvalidType
	}
	return c.send(ctx, req)
}

// SendOne sends a single metric via UpdateMetrics.
func (c *Client) SendOne(ctx context.Context, item domain.Metrics) error {
	pm, ok := metricToProto(item)
	if !ok {
		return domain.ErrInvalidType
	}
	return c.send(ctx, &pb.UpdateMetricsRequest{Metrics: []*pb.Metric{pm}})
}

func (c *Client) send(ctx context.Context, req *pb.UpdateMetricsRequest) error {
	if c == nil || c.client == nil {
		return errors.New("grpc client not initialized")
	}
	if c.realIP != "" {
		ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-real-ip", c.realIP))
	}
	_, err := c.client.UpdateMetrics(ctx, req)
	return err
}

func metricToProto(m domain.Metrics) (*pb.Metric, bool) {
	switch m.MType {
	case string(domain.Gauge):
		if m.Value == nil {
			return nil, false
		}
		return &pb.Metric{
			Id:    m.ID,
			Type:  pb.Metric_GAUGE,
			Value: *m.Value,
		}, true
	case string(domain.Counter):
		if m.Delta == nil {
			return nil, false
		}
		return &pb.Metric{
			Id:    m.ID,
			Type:  pb.Metric_COUNTER,
			Delta: *m.Delta,
		}, true
	default:
		return nil, false
	}
}

func localIP() string {
	return localIPFromAddrs(allInterfaceAddrs())
}

func localIPFromAddrs(addrs []net.Addr) string {
	if ip := selectIP(addrs, func(ip net.IP) bool {
		ip = ip.To4()
		return ip != nil && !ip.IsLoopback()
	}); ip != nil {
		return ip.String()
	}
	if ip := selectIP(addrs, func(ip net.IP) bool {
		ip = ip.To4()
		return ip != nil && ip.IsLoopback()
	}); ip != nil {
		return ip.String()
	}
	return "127.0.0.1"
}

func allInterfaceAddrs() []net.Addr {
	var out []net.Addr
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		out = append(out, addrs...)
	}
	return out
}

func selectIP(addrs []net.Addr, accept func(net.IP) bool) net.IP {
	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		default:
		}
		if ip == nil {
			continue
		}
		if accept(ip) {
			return ip
		}
	}
	return nil
}
