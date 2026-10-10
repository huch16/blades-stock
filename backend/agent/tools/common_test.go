package tools

import (
	"strings"
	"testing"
)

func TestSanitizeSensitiveInfo(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		contains string
		excludes string
	}{
		{
			name:     "OpenAI Key Redaction",
			input:    "model called with key sk-proj-1234567890abcdef123456 in header",
			contains: "sk-***REDACTED***",
			excludes: "sk-proj-1234567890abcdef123456",
		},
		{
			name:     "Bearer Token Redaction",
			input:    "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.xyz",
			contains: "Bearer ***REDACTED***",
			excludes: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
		},
		{
			name:     "URL Query Param Redaction",
			input:    "GET https://api.example.com/v1/search?api_key=mysecretkey123456&query=AAPL",
			contains: "api_key=***REDACTED***",
			excludes: "mysecretkey123456",
		},
		{
			name:     "Safe text unchanged",
			input:    "query stock price for 600519.SH",
			contains: "query stock price for 600519.SH",
			excludes: "***REDACTED***",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			output := SanitizeSensitiveInfo(tc.input)
			if !strings.Contains(output, tc.contains) {
				t.Errorf("expected output to contain %q, got %q", tc.contains, output)
			}
			if tc.excludes != "" && strings.Contains(output, tc.excludes) {
				t.Errorf("expected output NOT to contain %q, got %q", tc.excludes, output)
			}
		})
	}
}
