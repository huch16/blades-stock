package data

import (
	"testing"
)

func TestParseClosePrice(t *testing.T) {
	tests := []struct {
		input string
		want  float64
		err   bool
	}{
		{"10.50", 10.50, false},
		{"0", 0, false},
		{"-1.5", -1.5, false},
		{"", 0, false},
		{"-", 0, false},
		{"null", 0, false},
		{"  12.34  ", 12.34, false},
		{"abc", 0, true},
		{"1.2.3", 0, true},
	}
	for _, tc := range tests {
		got, err := ParseClosePrice(tc.input)
		if tc.err && err == nil {
			t.Errorf("ParseClosePrice(%q) expected error, got %v", tc.input, got)
		}
		if !tc.err && err != nil {
			t.Errorf("ParseClosePrice(%q) unexpected error: %v", tc.input, err)
		}
		if got != tc.want {
			t.Errorf("ParseClosePrice(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}
