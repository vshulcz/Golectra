package main

import (
	"flag"
	"io"
	"os"
	"strconv"
)

func applyServerFlags(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	if out == nil {
		out = io.Discard
	}
	fs.SetOutput(out)

	addr := fs.String("a", "", "HTTP listen address, default: localhost:8080")
	ival := fs.Int("i", -1, "STORE_INTERVAL seconds (0 - sync), default: 300")
	file := fs.String("f", "", "FILE_STORAGE_PATH, default: metrics-db.json")
	fs.Bool("r", false, "RESTORE on start (true/false), default: false")

	if err := fs.Parse(args); err != nil {
		return err
	}

	setIfEmpty := func(k, v string) {
		if os.Getenv(k) == "" && v != "" {
			_ = os.Setenv(k, v)
		}
	}

	if *addr != "" {
		setIfEmpty("ADDRESS", *addr)
	}
	if *ival >= 0 {
		setIfEmpty("STORE_INTERVAL", strconv.Itoa(*ival))
	}
	if *file != "" {
		setIfEmpty("FILE_STORAGE_PATH", *file)
	}
	if fs.Parsed() && fs.Lookup("r").Value.String() == "true" {
		setIfEmpty("RESTORE", "true")
	}
	return nil
}
