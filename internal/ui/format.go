package ui

import (
	"strconv"
	"strings"
	"time"
)

const (
	unitStep = 1024.0
	maxUnit  = 4
)

var byteUnits = [...]string{"B", "KB", "MB", "GB", "TB"}

// HumanSize renders a byte count as a short human-readable string (base-1024),
// e.g. 48.2 MB. Bytes render without a decimal.
func HumanSize(n int64) string {
	if n < int64(unitStep) {
		return strconv.FormatInt(n, 10) + " " + byteUnits[0]
	}
	value := float64(n)
	unit := 0
	for value >= unitStep && unit < maxUnit {
		value /= unitStep
		unit++
	}
	formatted := strconv.FormatFloat(value, 'f', 1, 64)
	formatted = strings.TrimSuffix(formatted, ".0")
	return formatted + " " + byteUnits[unit]
}

// HumanDuration renders an elapsed duration in the compact form the live log
// uses (e.g. 22s, 3m04s, 1h02m), rounding sub-second values up to 1s.
func HumanDuration(d time.Duration) string {
	if d < time.Second {
		d = time.Second
	}
	d = d.Round(time.Second)
	switch {
	case d < time.Minute:
		return strconv.Itoa(int(d.Seconds())) + "s"
	case d < time.Hour:
		m := int(d / time.Minute)
		s := int((d % time.Minute) / time.Second)
		return strconv.Itoa(m) + "m" + pad2(s) + "s"
	default:
		h := int(d / time.Hour)
		m := int((d % time.Hour) / time.Minute)
		return strconv.Itoa(h) + "h" + pad2(m) + "m"
	}
}

func pad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}
