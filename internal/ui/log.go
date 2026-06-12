package ui

import (
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/ul0gic/orbital/internal/events"
)

const (
	labelWidth = 8
	labelGap   = "   "
	tsLayout   = "15:04:05"
	inboxDir   = "orbital-inbox/"
)

// Log subscribes to the bus and renders each event as one append-only line in
// the frozen OI-2 format until the subscription is cancelled. It blocks; run it
// in its own goroutine. Cancelling the subscription (via the bus) ends the loop.
func (u *UI) Log(ch <-chan events.Event) {
	starts := make(map[string]time.Time)
	for e := range ch {
		u.out.write(u.renderLine(&e, starts) + "\n")
	}
}

func (u *UI) renderLine(e *events.Event, starts map[string]time.Time) string {
	glyph, label, sty := u.classify(e.Type)
	msg := u.message(e, starts)

	ts := u.pal.ts.Render(e.Time.Format(tsLayout))
	tag := sty.Render(glyph + " " + pad(label, labelWidth))
	return ts + " " + tag + labelGap + msg
}

func (u *UI) classify(t events.Type) (glyph, label string, sty lipgloss.Style) {
	switch t {
	case events.Ready:
		return "●", "ready", u.pal.ready
	case events.Visitor:
		return "◉", "visitor", u.pal.visitor
	case events.DownloadStart:
		return "↓", "download", u.pal.download
	case events.DownloadComplete:
		return "✓", "download", u.pal.ok
	case events.UploadStart:
		return "↑", "upload", u.pal.upload
	case events.UploadComplete:
		return "✓", "upload", u.pal.ok
	case events.UploadRejected:
		return "✗", "upload", u.pal.fail
	case events.Error:
		return "✗", "error", u.pal.fail
	default:
		return "·", "event", u.pal.dim
	}
}

func (u *UI) message(e *events.Event, starts map[string]time.Time) string {
	file := sanitizeForTerminal(e.File)
	client := sanitizeForTerminal(e.Client)
	errMsg := sanitizeForTerminal(e.Err)

	switch e.Type {
	case events.Ready:
		return file
	case events.Visitor:
		return "page opened (" + client + ")"
	case events.DownloadStart:
		starts[transferKey(e)] = e.Time
		return file + " (" + HumanSize(e.Size) + ") started"
	case events.DownloadComplete:
		return file + " (" + HumanSize(e.Size) + ") complete in " + elapsed(e, starts)
	case events.UploadStart:
		return file + " (" + HumanSize(e.Size) + ") started"
	case events.UploadComplete:
		return file + " " + u.pal.dim.Render("→") + " " + inboxDir + " complete"
	case events.UploadRejected:
		return file + " rejected (" + errMsg + ")"
	case events.Error:
		return errMsg
	default:
		return file
	}
}

func elapsed(e *events.Event, starts map[string]time.Time) string {
	k := transferKey(e)
	if start, ok := starts[k]; ok {
		delete(starts, k)
		return HumanDuration(e.Time.Sub(start))
	}
	return HumanDuration(0)
}

func transferKey(e *events.Event) string {
	return e.Client + "\x00" + e.File
}
