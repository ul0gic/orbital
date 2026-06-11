package ui

import "strings"

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func pad(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
