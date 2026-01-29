package metrics

import (
	"testing"

	"google.golang.org/protobuf/proto"
)

func TestMetricRoundTrip(t *testing.T) {
	orig := &Metric{Id: "Alloc", Type: Metric_GAUGE, Value: 1.23}
	data, err := proto.Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	var decoded Metric
	if err := proto.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded.GetId() != orig.GetId() {
		t.Fatalf("Id=%q want %q", decoded.GetId(), orig.GetId())
	}
	if decoded.GetType() != Metric_GAUGE {
		t.Fatalf("Type=%v want %v", decoded.GetType(), Metric_GAUGE)
	}
	if decoded.GetValue() != orig.GetValue() {
		t.Fatalf("Value=%v want %v", decoded.GetValue(), orig.GetValue())
	}
}

func TestMetricCounterRoundTrip(t *testing.T) {
	orig := &Metric{Id: "PollCount", Type: Metric_COUNTER, Delta: 7}
	data, err := proto.Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	var decoded Metric
	if err := proto.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded.GetId() != orig.GetId() {
		t.Fatalf("Id=%q want %q", decoded.GetId(), orig.GetId())
	}
	if decoded.GetType() != Metric_COUNTER {
		t.Fatalf("Type=%v want %v", decoded.GetType(), Metric_COUNTER)
	}
	if decoded.GetDelta() != orig.GetDelta() {
		t.Fatalf("Delta=%v want %v", decoded.GetDelta(), orig.GetDelta())
	}
}

func TestRequestResponse(t *testing.T) {
	req := &UpdateMetricsRequest{Metrics: []*Metric{{Id: "Alloc", Type: Metric_GAUGE, Value: 2}}}
	if len(req.GetMetrics()) != 1 {
		t.Fatalf("metrics len=%d want 1", len(req.GetMetrics()))
	}
	resp := &UpdateMetricsResponse{}
	if proto.Size(resp) != 0 {
		t.Fatal("expected empty response to have zero size")
	}
}

func TestGeneratedMethods(t *testing.T) {
	m := &Metric{Id: "M", Type: Metric_COUNTER, Delta: 1}
	m.Reset()
	_ = m.String()
	_ = m.ProtoReflect()
	_, _ = m.Descriptor()
	_ = m.GetId()
	_ = m.GetType()
	_ = m.GetDelta()
	_ = m.GetValue()

	req := &UpdateMetricsRequest{}
	req.Reset()
	_ = req.String()
	_ = req.ProtoReflect()
	_, _ = req.Descriptor()

	resp := &UpdateMetricsResponse{}
	resp.Reset()
	_ = resp.String()
	_ = resp.ProtoReflect()
	_, _ = resp.Descriptor()

	_ = Metric_GAUGE.Enum()
	_ = Metric_GAUGE.String()
	_ = Metric_GAUGE.Descriptor()
	_ = Metric_GAUGE.Type()
	_ = Metric_GAUGE.Number()

	_ = Metric_COUNTER.Enum()
	_ = Metric_COUNTER.String()
	_ = Metric_COUNTER.Descriptor()
	_ = Metric_COUNTER.Type()
	_ = Metric_COUNTER.Number()

	_ = Metric_MType_name[int32(Metric_GAUGE)]
	_ = Metric_MType_value[Metric_COUNTER.String()]
	_ = file_metrics_proto_rawDescGZIP()
	_ = File_metrics_proto
}
