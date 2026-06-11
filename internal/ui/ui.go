package ui

import "os"

// UI renders all non-data terminal output (banner, QR, live log) to a single
// diagnostic writer. The share URL is the caller's responsibility (stdout).
type UI struct {
	out *Writer
	pal palette
}

// New builds a UI over the given diagnostic writer. Color and QR are enabled
// only when the writer is a TTY and NO_COLOR is unset.
func New(out *Writer) *UI {
	return &UI{
		out: out,
		pal: newPalette(out.IsTTY() && !noColor()),
	}
}

func noColor() bool {
	_, set := os.LookupEnv("NO_COLOR")
	return set
}
