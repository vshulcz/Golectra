package domain

// MetricType enumerates supported metric transports.
type MetricType string

const (
	// Gauge represents a floating-point value that can move up or down.
	Gauge MetricType = "gauge"
	// Counter represents a monotonically increasing integer.
	Counter MetricType = "counter"
)

// Metrics describes a single gauge or counter payload.
type Metrics struct {
	Delta *int64   `json:"delta,omitempty"`
	Value *float64 `json:"value,omitempty"`
	ID    string   `json:"id"`
	MType string   `json:"type"`
}

// Snapshot groups all currently known gauge and counter values.
type Snapshot struct {
	Gauges   map[string]float64
	Counters map[string]int64
}
