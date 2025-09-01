package agent

import "time"

type Agent interface {
	Start()
	Stop()
}

type Config struct {
	Address        string
	PollInterval   time.Duration
	ReportInterval time.Duration
}
