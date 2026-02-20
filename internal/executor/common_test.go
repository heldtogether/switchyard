package executor

import (
	"testing"
)

func TestParseCPU(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{"one core", "1.0", 1000000000},
		{"half core", "0.5", 500000000},
		{"two cores", "2.0", 2000000000},
		{"quarter core", "0.25", 250000000},
		{"integer", "1", 1000000000},
		{"four cores", "4", 4000000000},
		{"empty string", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseCPU(tt.input)
			if result != tt.expected {
				t.Errorf("ParseCPU(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseMemory(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{"2 gigabytes", "2g", 2 * 1024 * 1024 * 1024},
		{"2 gigabytes uppercase", "2G", 2 * 1024 * 1024 * 1024},
		{"2 gigabytes with b", "2gb", 2 * 1024 * 1024 * 1024},
		{"512 megabytes", "512m", 512 * 1024 * 1024},
		{"512 megabytes uppercase", "512M", 512 * 1024 * 1024},
		{"512 megabytes with b", "512mb", 512 * 1024 * 1024},
		{"1024 kilobytes", "1024k", 1024 * 1024},
		{"1024 kilobytes uppercase", "1024K", 1024 * 1024},
		{"1024 kilobytes with b", "1024kb", 1024 * 1024},
		{"raw bytes", "1024", 1024},
		{"with spaces", " 2g ", 2 * 1024 * 1024 * 1024},
		{"empty string", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseMemory(tt.input)
			if result != tt.expected {
				t.Errorf("ParseMemory(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractJobIDFromLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		expected string
	}{
		{
			name: "job id present",
			labels: map[string]string{
				"jobrunner.job_id":  "550e8400-e29b-41d4-a716-446655440000",
				"jobrunner.managed": "true",
			},
			expected: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name: "job id missing",
			labels: map[string]string{
				"jobrunner.managed": "true",
			},
			expected: "",
		},
		{
			name:     "nil labels",
			labels:   nil,
			expected: "",
		},
		{
			name:     "empty labels",
			labels:   map[string]string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractJobIDFromLabels(tt.labels)
			if result != tt.expected {
				t.Errorf("ExtractJobIDFromLabels(%v) = %q, want %q", tt.labels, result, tt.expected)
			}
		})
	}
}
