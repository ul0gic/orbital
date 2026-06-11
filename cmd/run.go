package cmd

import (
	"context"

	"github.com/ul0gic/sidedrop/internal/ui"
)

// Session is handed to the injected core so it can drive terminal output
// without owning stdout/stderr policy. The share URL is the only thing that
// reaches stdout, and only through ShareReady.
type Session struct {
	UI *ui.UI

	stdout    stringWriter
	cfg       Config
	announced bool
}

type stringWriter interface {
	WriteString(s string) (int, error)
}

// ShareReady is called by the core exactly once when the public URL exists.
// fileCount is the number of files being served (known after the manifest is
// built). It prints the URL to stdout — the sole stdout write, so
// `sidedrop | pbcopy` captures only the link — and renders the banner + QR to
// stderr.
func (s *Session) ShareReady(url string, fileCount int) {
	if s.announced {
		return
	}
	s.announced = true

	s.UI.Banner(ui.BannerInfo{
		Path:      s.cfg.Path,
		FileCount: fileCount,
		ShareURL:  url,
		NoUpload:  s.cfg.NoUpload,
	})
	s.UI.QR(url)

	_, _ = s.stdout.WriteString(url + "\n")
}

// RunFunc is the seam main.go wires to the core. cmd parses and validates the
// Config, constructs the Session, and invokes this; the core serves files,
// starts the tunnel, and calls Session.ShareReady when the link is live.
type RunFunc func(ctx context.Context, cfg Config, sess *Session) error
