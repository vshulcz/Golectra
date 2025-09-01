package agent

import "time"

type Agent interface {
	Start()
	Stop()
}

type Config struct {
	ServerURL      string
	PollInterval   time.Duration
	ReportInterval time.Duration
}
