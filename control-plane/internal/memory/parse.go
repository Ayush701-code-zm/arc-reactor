package memory

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

func ParseSize(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("memory size is empty")
	}

	i := 0
	for i < len(s) && (unicode.IsDigit(rune(s[i])) || s[i] == '.') {
		i++
	}
	if i == 0 {
		return 0, fmt.Errorf("memory size %q has no number", s)
	}

	num, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return 0, fmt.Errorf("memory size %q: %w", s, err)
	}

	unit := strings.ToUpper(strings.TrimSpace(s[i:]))
	var mult float64
	switch unit {
	case "", "B":
		mult = 1
	case "K", "KB":
		mult = 1024
	case "M", "MB":
		mult = 1024 * 1024
	case "G", "GB":
		mult = 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("memory size %q: unknown unit %q", s, unit)
	}

	if num <= 0 {
		return 0, fmt.Errorf("memory size must be > 0")
	}
	return uint64(num * mult), nil
}
