package main

import (
	"testing"

	"golang.org/x/tools/go/analysis"
)

func TestFilterAnalyzers(t *testing.T) {
	tests := []struct {
		name     string
		input    []*analysis.Analyzer
		expected int
	}{
		{
			name: "filter Golectra analyzers",
			input: []*analysis.Analyzer{
				{Name: "GolectraAnalyzer1"},
				{Name: "GolectraAnalyzer2"},
				{Name: "OtherAnalyzer"},
			},
			expected: 2,
		},
		{
			name: "no Golectra analyzers",
			input: []*analysis.Analyzer{
				{Name: "OtherAnalyzer1"},
				{Name: "OtherAnalyzer2"},
			},
			expected: 0,
		},
		{
			name:     "empty input",
			input:    []*analysis.Analyzer{},
			expected: 0,
		},
		{
			name: "all Golectra analyzers",
			input: []*analysis.Analyzer{
				{Name: "GolectraAnalyzer1"},
				{Name: "GolectraAnalyzer2"},
			},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := filterAnalyzers(tt.input)

			if len(filtered) != tt.expected {
				t.Errorf("expected %d analyzers, got %d", tt.expected, len(filtered))
			}

			for _, a := range filtered {
				if a.Name != "GolectraAnalyzer1" && a.Name != "GolectraAnalyzer2" {
					t.Errorf("unexpected analyzer: %s", a.Name)
				}
			}
		})
	}
}
