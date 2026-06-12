package ui

import "strings"

const replacementChar = '�'

// sanitizeForTerminal replaces every control character that could drive a
// terminal escape sequence with the Unicode replacement char: C0 (< 0x20), DEL
// (0x7f), and C1 (0x80–0x9f). Untrusted event fields pass through this before
// they enter a styled log line, so an uploaded filename or spoofed client hint
// cannot rewrite the operator's terminal.
func sanitizeForTerminal(s string) string {
	if !strings.ContainsFunc(s, isTerminalControl) {
		return s
	}
	return strings.Map(func(r rune) rune {
		if isTerminalControl(r) {
			return replacementChar
		}
		return r
	}, s)
}

func isTerminalControl(r rune) bool {
	return r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f)
}
