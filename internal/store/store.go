package store

type MetricType string

const (
	Gauge   MetricType = "gauge"
	Counter MetricType = "counter"
)

type Storage interface {
	GetGauge(name string) (float64, bool)
	GetCounter(name string) (int64, bool)
	UpdateGauge(name string, value float64) error
	UpdateCounter(name string, delta int64) error
}