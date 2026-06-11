package ui

import (
	"io"
	"os"

	"golang.org/x/term"
)

// Writer is the diagnostic sink for everything that is not the share URL:
// banner, QR, and the live event log. It tracks whether its destination is a
// TTY so callers can drop color and the QR block when output is piped.
type Writer struct {
	w     io.Writer
	isTTY bool
}

// NewStderrWriter targets os.Stderr and detects whether it is a terminal.
// The share URL is never written here — it goes to stdout via cmd.
func NewStderrWriter() *Writer {
	return &Writer{
		w:     os.Stderr,
		isTTY: isTerminal(os.Stderr),
	}
}

// NewWriter wraps an arbitrary destination; isTTY controls color and QR
// suppression. Used by tests to exercise both terminal and piped behavior.
func NewWriter(w io.Writer, isTTY bool) *Writer {
	return &Writer{w: w, isTTY: isTTY}
}

// IsTTY reports whether the destination is an interactive terminal.
func (wr *Writer) IsTTY() bool { return wr.isTTY }

func (wr *Writer) write(s string) {
	_, _ = io.WriteString(wr.w, s)
}

func isTerminal(f *os.File) bool {
	fd := f.Fd()
	//nolint:gosec // os file descriptors are small non-negative values; the uintptr->int narrowing cannot overflow
	return term.IsTerminal(int(fd))
}
