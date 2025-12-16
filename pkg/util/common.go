// Package util provides utility functions for the application.
package util

import "fmt"

// na returns "N/A" if the input string is empty, otherwise it returns the input string.
func na(v string) string {
	if v == "" {
		return "N/A"
	}
	return v
}

// PrintBuildInfo prints the build version, date, and commit information.
func PrintBuildInfo(buildVersion, buildDate, buildCommit string) {
	fmt.Printf("Build version: %s\n", na(buildVersion))
	fmt.Printf("Build date: %s\n", na(buildDate))
	fmt.Printf("Build commit: %s\n", na(buildCommit))
}
