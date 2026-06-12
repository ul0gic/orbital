package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ul0gic/orbital/internal/events"
)

func TestSanitizeForTerminalReplacesControlBytes(t *testing.T) {
	in := "a\x1bb\x00c\rd\ae\x7ff\x9bg"
	got := sanitizeForTerminal(in)
	for _, r := range got {
		if isTerminalControl(r) {
			t.Fatalf("sanitizeForTerminal left control rune %#x in %q", r, got)
		}
	}
	if strings.ContainsAny(got, "abcdefg") == false {
		t.Errorf("sanitizeForTerminal dropped printable content: %q", got)
	}
}

func TestSanitizeForTerminalPassesCleanString(t *testing.T) {
	in := "photos.zip → orbital-inbox/ complete"
	if got := sanitizeForTerminal(in); got != in {
		t.Errorf("clean string altered: %q -> %q", in, got)
	}
}

// TestLogRendersNoRawControlBytes is the SEC-002 regression: an event carrying
// escape sequences in File and Client must render with zero raw control bytes.
// The non-TTY writer disables lipgloss styling, so any control byte in the
// output came from the untrusted data, not the renderer.
func TestLogRendersNoRawControlBytes(t *testing.T) {
	evs := []events.Event{
		{
			Type:   events.UploadRejected,
			Time:   time.Unix(0, 0),
			File:   "\x1b]0;PWNED\a\x1b[2J malicious.txt",
			Client: "Chrome, \x1b]0;TITLE\a203.0.113.7",
			Err:    "Bad Request\x1b[2K",
		},
		{
			Type:   events.Visitor,
			Time:   time.Unix(0, 0),
			Client: "Chrome, \x00\r\a203.0.113.7",
		},
		{
			Type: events.UploadStart,
			Time: time.Unix(0, 0),
			File: "a\x1bb\x00c.txt",
		},
	}

	var buf bytes.Buffer
	u := New(NewWriter(&buf, false))
	u.Log(channelOf(evs...))

	out := buf.String()
	for i, r := range out {
		if r == '\n' {
			continue
		}
		if isTerminalControl(r) {
			t.Fatalf("raw control rune %#x at offset %d in rendered log:\n%q", r, i, out)
		}
	}
}

func channelOf(es ...events.Event) <-chan events.Event {
	ch := make(chan events.Event, len(es))
	for _, e := range es {
		ch <- e
	}
	close(ch)
	return ch
}
