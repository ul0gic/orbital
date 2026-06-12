package server

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	prefix := "/" + s.token

	mux.HandleFunc(prefix+"/{$}", s.methodGate(http.MethodGet, s.handleLanding))
	mux.HandleFunc(prefix+"/manifest.json", s.methodGate(http.MethodGet, s.handleManifest))
	mux.HandleFunc(prefix+"/f/", s.methodGate(http.MethodGet, s.handleDownload))
	if s.cfg.Upload != nil {
		upload := http.StripPrefix(prefix, s.cfg.Upload).ServeHTTP
		mux.HandleFunc(prefix+"/upload", s.methodGate(http.MethodPost, upload))
	}

	return s.tokenGuard(mux)
}

// methodGate enforces the single intended method for a route. Any other method
// gets the same bare 404 as an unknown path, so a method mismatch never
// confirms that the route exists.
func (s *Server) methodGate(method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			http.NotFound(w, r)
			return
		}
		next(w, r)
	}
}

// tokenGuard answers any request whose path does not begin with the exact
// session token with a bare 404 that confirms nothing about what exists. The
// token segment is compared in constant time so the path secret leaks no
// timing signal.
func (s *Server) tokenGuard(next http.Handler) http.Handler {
	want := []byte(s.token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seg, rest := tokenSegment(r.URL.Path)
		if len(seg) != len(want) || subtle.ConstantTimeCompare([]byte(seg), want) != 1 {
			http.NotFound(w, r)
			return
		}
		if rest != "" && !strings.HasPrefix(rest, "/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// tokenSegment splits /{token}/... into the token segment and the remainder
// (including the leading slash, or "" for a bare /{token}).
func tokenSegment(path string) (segment, rest string) {
	if path == "" || path[0] != '/' {
		return "", path
	}
	path = path[1:]
	if i := strings.IndexByte(path, '/'); i >= 0 {
		return path[:i], path[i:]
	}
	return path, ""
}
