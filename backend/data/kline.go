package data

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseClosePrice parses a KLineData.Close string into float64.
// Returns 0, nil for empty/missing values (common in API responses).
func ParseClosePrice(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" || s == "null" {
		return 0, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parse close price %q: %w", s, err)
	}
	return v, nil
}
