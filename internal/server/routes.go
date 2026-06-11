package server

import (
	"net/http"
	"strings"
)

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	prefix := "/" + s.token

	mux.HandleFunc("GET "+prefix+"/", s.handleLanding)
	mux.HandleFunc("GET "+prefix+"/manifest.json", s.handleManifest)
	mux.HandleFunc("GET "+prefix+"/f/", s.handleDownload)
	if s.cfg.Upload != nil {
		mux.Handle("POST "+prefix+"/upload", http.StripPrefix(prefix, s.cfg.Upload))
	}

	return s.tokenGuard(mux)
}

// tokenGuard answers any request whose path does not begin with the exact
// session token with a bare 404 that confirms nothing about what exists.
func (s *Server) tokenGuard(next http.Handler) http.Handler {
	prefix := "/" + s.token
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != prefix && !strings.HasPrefix(r.URL.Path, prefix+"/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
