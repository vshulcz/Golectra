package main

import (
	"flag"
	"io"
	"os"
)

func applyServerFlags(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	if out == nil {
		out = io.Discard
	}
	fs.SetOutput(out)

	addr := fs.String("a", "", "HTTP listen address, default: localhost:8080")

	if err := fs.Parse(args); err != nil {
		return err
	}
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "a" && *addr != "" {
			os.Setenv("ADDRESS", *addr)
		}
	})
	return nil
}
