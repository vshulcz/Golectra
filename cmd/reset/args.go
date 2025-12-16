package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// findModuleRoot finds the root directory of the Go module by looking for the go.mod file upwards from the current working directory.
func findModuleRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("go.mod not found upwards from %s", wd)
}
