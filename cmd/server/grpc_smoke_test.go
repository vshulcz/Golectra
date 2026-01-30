package main

import "testing"

func TestServeGRPC_Nil(t *testing.T) {
	if err := serveGRPC(nil, nil); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestStopGRPC_Nil(t *testing.T) {
	stopGRPC(nil, nil)
}
