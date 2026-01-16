package agent

import "time"

// Config holds runtime parameters needed by the agent application service.
type Config struct {
	PollInterval   time.Duration
	ReportInterval time.Duration
	RateLimit      int
}
