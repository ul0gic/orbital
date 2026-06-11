package cmd

import (
	"errors"
	"strconv"
	"strings"
)

var errBadSize = errors.New("invalid size: use forms like 500MB, 2GB, 512MiB, or a plain byte count")

var sizeUnits = []struct {
	suffix string
	mult   int64
}{
	{"KIB", 1 << 10}, {"MIB", 1 << 20}, {"GIB", 1 << 30}, {"TIB", 1 << 40},
	{"KB", 1000}, {"MB", 1000 * 1000}, {"GB", 1000 * 1000 * 1000}, {"TB", 1000 * 1000 * 1000 * 1000},
	{"K", 1 << 10}, {"M", 1 << 20}, {"G", 1 << 30}, {"T", 1 << 40},
	{"B", 1},
}

// parseSize converts a human size string (500MB, 2GB, 512MiB, 1048576) into a
// byte count. Decimal suffixes are powers of 1000; binary suffixes (and bare
// K/M/G/T) are powers of 1024. Negative or zero results are rejected.
func parseSize(s string) (int64, error) {
	raw := strings.TrimSpace(strings.ToUpper(s))
	if raw == "" {
		return 0, errBadSize
	}

	mult := int64(1)
	num := raw
	for _, u := range sizeUnits {
		if strings.HasSuffix(raw, u.suffix) {
			mult = u.mult
			num = strings.TrimSpace(strings.TrimSuffix(raw, u.suffix))
			break
		}
	}

	value, err := strconv.ParseFloat(num, 64)
	if err != nil || value <= 0 {
		return 0, errBadSize
	}

	bytes := int64(value * float64(mult))
	if bytes <= 0 {
		return 0, errBadSize
	}
	return bytes, nil
}
