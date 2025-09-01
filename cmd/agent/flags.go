package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	DefaultServerAddr     = "http://localhost:8080"
	DefaultReportInterval = 10
	DefaultPollInterval   = 2
)

func applyAgentFlags(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("agent", flag.ContinueOnError)
	if out == nil {
		out = io.Discard
	}
	fs.SetOutput(out)

	addr := fs.String("a", "", fmt.Sprintf("server address (host:port or URL), default: %s", DefaultServerAddr))
	report := fs.Int("r", 0, fmt.Sprintf("report interval in seconds, default: %d", DefaultReportInterval))
	poll := fs.Int("p", 0, fmt.Sprintf("poll interval in seconds, default: %d", DefaultPollInterval))

	if err := fs.Parse(args); err != nil {
		return err
	}

	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "a":
			v := *addr
			if !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") {
				v = "http://" + v
			}
			os.Setenv("ADDRESS", v)
		case "r":
			if *report > 0 {
				os.Setenv("REPORT_INTERVAL", fmt.Sprintf("%ds", *report))
			}
		case "p":
			if *poll > 0 {
				os.Setenv("POLL_INTERVAL", fmt.Sprintf("%ds", *poll))
			}
		}
	})

	return nil
}
