package main

import (
	"testing"
)

func TestBuildVariablesExist(t *testing.T) {
	_ = buildVersion
	_ = buildDate
	_ = buildCommit
}
