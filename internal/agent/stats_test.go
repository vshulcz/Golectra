package agent

import (
	"testing"
)

func TestStats_SetAndSnapshot(t *testing.T) {
	s := newStats()

	s.setGauge("Alloc", 123.45)
	s.addCounter("PollCount", 2)
	s.addCounter("PollCount", 3)

	g, c := s.snapshot()

	if g["Alloc"] != 123.45 {
		t.Errorf("expected gauge=123.45, got %v", g["Alloc"])
	}
	if c["PollCount"] != 5 {
		t.Errorf("expected counter=5, got %v", c["PollCount"])
	}

	g["Alloc"] = 0
	c["PollCount"] = 0

	g2, c2 := s.snapshot()
	if g2["Alloc"] != 123.45 {
		t.Errorf("snapshot should be independent, got %v", g2["Alloc"])
	}
	if c2["PollCount"] != 5 {
		t.Errorf("snapshot should be independent, got %v", c2["PollCount"])
	}
}
