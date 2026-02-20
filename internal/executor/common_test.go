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
		// Edge cases
		{"zero", "0", 0},
		{"zero with decimal", "0.0", 0},
		{"very small", "0.01", 10000000},
		{"very large", "100", 100000000000},
		{"with whitespace", " 1.0 ", 1000000000},
		{"with tabs", "\t2.0\t", 2000000000},
		{"invalid format", "abc", 0},
		// Note: strconv.ParseFloat accepts these, so they parse "successfully"
		{"invalid decimal", "1.2.3", 1200000000},      // parses as 1.2
		{"double dot", "1..0", 1000000000},            // parses as 1.
		{"negative", "-1.0", -1000000000},             // negative values allowed by ParseFloat
		{"multiple decimals", "1.234567", 1234567000}, // full precision
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
		// Edge cases
		{"zero", "0", 0},
		{"zero gigabytes", "0g", 0},
		{"very large", "100g", 100 * 1024 * 1024 * 1024},
		// Note: Decimal parsing works by strconv.Atoi on the number part, so decimals truncate
		{"decimal gigabytes", "1.5g", 1},     // atoi("1.5") fails, gets just prefix
		{"decimal megabytes", "256.5m", 256}, // atoi("256.5") fails, gets just prefix
		{"with tabs", "\t512m\t", 512 * 1024 * 1024},
		{"mixed case GB", "2Gb", 2 * 1024 * 1024 * 1024},
		{"mixed case MB", "512Mb", 512 * 1024 * 1024},
		// Note: Invalid units fall through to parsing as bytes
		{"invalid unit", "512x", 512},   // parsed as raw bytes (number only)
		{"invalid terabytes", "2tb", 2}, // parsed as raw bytes
		{"invalid format", "abc", 0},
		{"negative", "-512m", -536870912}, // negative values allowed
		{"just unit", "g", 0},
		{"double unit", "512gg", 512}, // parsed as raw bytes
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
