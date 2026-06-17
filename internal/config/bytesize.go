package config

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ByteSize is an int64 that can be unmarshalled from a human-readable string
// ("100MB", "1GB") or a plain integer (bytes).
type ByteSize int64

const (
	_KB ByteSize = 1024
	_MB ByteSize = 1024 * _KB
	_GB ByteSize = 1024 * _MB
)

func (b *ByteSize) UnmarshalYAML(value *yaml.Node) error {
	// Try plain integer first.
	var n int64
	if err := value.Decode(&n); err == nil {
		*b = ByteSize(n)
		return nil
	}
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := parseByteSize(s)
	if err != nil {
		return err
	}
	*b = parsed
	return nil
}

func parseByteSize(s string) (ByteSize, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return 0, nil
	}

	upper := strings.ToUpper(s)
	suffixes := []struct {
		suffix string
		mult   ByteSize
	}{
		{"GB", _GB},
		{"MB", _MB},
		{"KB", _KB},
		{"G", _GB},
		{"M", _MB},
		{"K", _KB},
		{"B", 1},
	}

	for _, sf := range suffixes {
		if strings.HasSuffix(upper, sf.suffix) {
			numStr := strings.TrimSpace(s[:len(s)-len(sf.suffix)])
			n, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid byte size %q: %w", s, err)
			}
			return ByteSize(n * float64(sf.mult)), nil
		}
	}

	// No suffix — treat as plain bytes.
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid byte size %q", s)
	}
	return ByteSize(n), nil
}

func (b ByteSize) String() string {
	switch {
	case b == 0:
		return "unlimited"
	case b%_GB == 0:
		return fmt.Sprintf("%dGB", b/_GB)
	case b%_MB == 0:
		return fmt.Sprintf("%dMB", b/_MB)
	case b%_KB == 0:
		return fmt.Sprintf("%dKB", b/_KB)
	default:
		return fmt.Sprintf("%dB", b)
	}
}
