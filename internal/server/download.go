package server

import (
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/ul0gic/orbital/internal/events"
)

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	rel := downloadRel(r.URL.Path, s.token)
	if rel == "" {
		http.NotFound(w, r)
		return
	}
	entry, ok := s.cfg.Manifest.Lookup(rel)
	if !ok {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(entry.AbsPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() { _ = f.Close() }()

	client := s.cfg.Visitors.Hint(r)
	s.publish(events.Event{
		Type:   events.DownloadStart,
		Time:   time.Now(),
		File:   rel,
		Size:   entry.Size,
		Client: client,
	})

	cw := &countWriter{ResponseWriter: w}
	http.ServeContent(cw, r, path.Base(rel), modTime(entry.AbsPath), f)

	s.publish(events.Event{
		Type:   events.DownloadComplete,
		Time:   time.Now(),
		File:   rel,
		Size:   entry.Size,
		Bytes:  cw.n,
		Client: client,
	})
}

// downloadRel extracts the manifest-relative path from /{token}/f/{rel}.
// r.URL.Path is already percent-decoded, so the result matches manifest keys
// directly.
func downloadRel(urlPath, token string) string {
	prefix := "/" + token + "/f/"
	if !strings.HasPrefix(urlPath, prefix) {
		return ""
	}
	return strings.TrimPrefix(urlPath, prefix)
}

func modTime(absPath string) time.Time {
	info, err := os.Stat(absPath)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

type countWriter struct {
	http.ResponseWriter
	n int64
}

func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.ResponseWriter.Write(p)
	c.n += int64(n)
	return n, err
}
