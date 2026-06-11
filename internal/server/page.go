package server

import (
	"encoding/json"
	"net/http"
	"path"
	"time"

	"github.com/ul0gic/sidedrop/internal/events"
)

const csp = "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data:; connect-src 'self'"

type manifestFile struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type manifestResponse struct {
	Files   []manifestFile `json:"files"`
	Uploads bool           `json:"uploads"`
}

func (s *Server) handleLanding(w http.ResponseWriter, r *http.Request) {
	s.publish(events.Event{
		Type:   events.Visitor,
		Time:   time.Now(),
		Client: clientHint(r),
	})
	w.Header().Set("Content-Security-Policy", csp)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(s.page)
}

func (s *Server) handleManifest(w http.ResponseWriter, _ *http.Request) {
	resp := manifestResponse{
		Files:   make([]manifestFile, 0, len(s.cfg.Manifest.Entries)),
		Uploads: s.cfg.Upload != nil,
	}
	for _, e := range s.cfg.Manifest.Entries {
		resp.Files = append(resp.Files, manifestFile{
			Name: path.Base(e.RelPath),
			Path: e.RelPath,
			Size: e.Size,
		})
	}
	w.Header().Set("Content-Security-Policy", csp)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		return
	}
}
