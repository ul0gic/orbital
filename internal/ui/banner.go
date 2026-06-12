package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// BannerInfo is the startup summary shown above the live log.
type BannerInfo struct {
	Path      string
	FileCount int
	ShareURL  string
	NoUpload  bool
}

// Banner writes the startup banner to the diagnostic writer (stderr). The
// share URL appears here for the human; the machine-readable copy is the
// caller's separate stdout write.
func (u *UI) Banner(info BannerInfo) {
	files := strconv.Itoa(info.FileCount) + " " + plural(info.FileCount, "file", "files")
	mode := "download + upload"
	if info.NoUpload {
		mode = "download only"
	}

	lines := []string{
		u.pal.title.Render("orbital") + "  " + u.pal.dim.Render("ephemeral file exchange"),
		u.pal.label.Render("serving  ") + info.Path + u.pal.dim.Render("  ("+files+", "+mode+")"),
		u.pal.label.Render("link     ") + u.pal.url.Render(info.ShareURL),
	}
	body := strings.Join(lines, "\n")

	if u.pal.enabled {
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("99")).
			Padding(0, 2)
		u.out.write(box.Render(body) + "\n\n")
		return
	}
	u.out.write(body + "\n\n")
}
