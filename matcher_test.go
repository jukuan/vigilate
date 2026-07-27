package main

import (
	"testing"
)

func TestPatternMatcher_Match(t *testing.T) {
	tests := []struct {
		name        string
		patterns    []string
		line        string
		wantMatched bool
		wantSample  string // expected sample substring
	}{
		{
			name:        "exact match",
			patterns:    []string{"FATAL"},
			line:        "FATAL: something broke",
			wantMatched: true,
			wantSample:  "FATAL: something",
		},
		{
			name:        "case insensitive",
			patterns:    []string{"error"},
			line:        "ERROR: disk full",
			wantMatched: true,
			wantSample:  "ERROR: disk full",
		},
		{
			name:        "regex special chars escaped",
			patterns:    []string{"[INFO]"},
			line:        "this [INFO] message",
			wantMatched: true,
			wantSample:  "is [INFO] messag",
		},
		{
			name:        "no match",
			patterns:    []string{"FATAL"},
			line:        "DEBUG: all good",
			wantMatched: false,
		},
		{
			name:        "multiple patterns, OR logic",
			patterns:    []string{"FATAL", "ERROR"},
			line:        "ERROR: something",
			wantMatched: true,
			wantSample:  "ERROR: something",
		},
		{
			name:        "pattern with regex (starts with)",
			patterns:    []string{"^start"},
			line:        "start of line",
			wantMatched: true,
			wantSample:  "start of line",
		},
		{
			name:        "pattern with regex (end with)",
			patterns:    []string{"end$"},
			line:        "this is the end",
			wantMatched: true,
			wantSample:  "end",
		},
		{
			name:        "short match gets context",
			patterns:    []string{"FATAL"},
			line:        "[2026-07-04] FATAL: disk failure imminent",
			wantMatched: true,
			wantSample:  "FATAL: disk fail",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := NewPatternMatcher(tt.patterns, 16)
			result := pm.Match(tt.line)
			if result.Matched != tt.wantMatched {
				t.Errorf("Match(%q).Matched = %v, want %v", tt.line, result.Matched, tt.wantMatched)
			}
			if tt.wantMatched && result.Sample != tt.wantSample {
				t.Errorf("Match(%q).Sample = %q, want %q", tt.line, result.Sample, tt.wantSample)
			}
		})
	}
}

func TestTruncateLogStr(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"short", "short"},
		{"this is a long string", "this is a long s"},
		{"", ""},
		{"exactly 16 chars!!", "exactly 16 chars"},
	}
	for _, tt := range tests {
		got := truncateLogStr(tt.input, 16)
		if got != tt.want {
			t.Errorf("truncateLogStr(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
