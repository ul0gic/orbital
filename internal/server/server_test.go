package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ul0gic/orbital/internal/events"
	"github.com/ul0gic/orbital/internal/manifest"
	"github.com/ul0gic/orbital/internal/upload"
)

type recorder struct {
	mu     sync.Mutex
	events []events.Event
}

//nolint:gocritic // matches the Publisher contract: Publish takes events.Event by value.
func (r *recorder) Publish(e events.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

func (r *recorder) has(t events.Type) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.events {
		if e.Type == t {
			return true
		}
	}
	return false
}

// fixture builds a server over a temp dir with the given files and optional
// upload support, returning the server and a live httptest server.
func fixture(t *testing.T, files map[string]string, withUpload bool) (srv *Server, ts *httptest.Server) {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	m, err := manifest.Build(dir, manifest.Options{})
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}

	var up http.Handler
	if withUpload {
		h, upErr := upload.New(upload.Config{Root: m.Root, MaxBytes: upload.DefaultMaxBytes})
		if upErr != nil {
			t.Fatalf("upload.New: %v", upErr)
		}
		up = h
	}

	srv, err = New(Config{Manifest: m, Events: &recorder{}, Upload: up})
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	ts = httptest.NewServer(srv.routes())
	t.Cleanup(ts.Close)
	return srv, ts
}

func buildManifest(t *testing.T, root string) *manifest.Manifest {
	t.Helper()
	m, err := manifest.Build(root, manifest.Options{})
	if err != nil {
		t.Fatalf("manifest.Build: %v", err)
	}
	return m
}

func writeServerFile(root, rel, content string) error {
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(content), 0o644)
}

// serverFor builds a server over an already-populated root with the given
// publisher, returning the server and a live httptest server.
func serverFor(t *testing.T, root string, pub Publisher, withUpload bool) (srv *Server, ts *httptest.Server) {
	t.Helper()
	m, err := manifest.Build(root, manifest.Options{})
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	var up http.Handler
	if withUpload {
		h, upErr := upload.New(upload.Config{Root: m.Root, MaxBytes: upload.DefaultMaxBytes, Events: pub})
		if upErr != nil {
			t.Fatalf("upload.New: %v", upErr)
		}
		up = h
	}
	srv, err = New(Config{Manifest: m, Events: pub, Upload: up})
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	ts = httptest.NewServer(srv.routes())
	t.Cleanup(ts.Close)
	return srv, ts
}

func get(t *testing.T, ts *httptest.Server, path string) *http.Response {
	t.Helper()
	resp, err := http.Get(ts.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func TestLandingServedWithCorrectToken(t *testing.T) {
	srv, ts := fixture(t, map[string]string{"a.txt": "x"}, false)
	resp := get(t, ts, "/"+srv.Token()+"/")
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Security-Policy"); !strings.Contains(got, "default-src 'none'") {
		t.Errorf("CSP header missing/weak: %q", got)
	}
}

func TestManifestJSON(t *testing.T) {
	srv, ts := fixture(t, map[string]string{"a.txt": "12345", "sub/b.txt": "ab"}, true)
	resp := get(t, ts, "/"+srv.Token()+"/manifest.json")
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body struct {
		Files []struct {
			Name string `json:"name"`
			Path string `json:"path"`
			Size int64  `json:"size"`
		} `json:"files"`
		Uploads bool `json:"uploads"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Uploads {
		t.Error("Uploads should be true when upload handler present")
	}
	if len(body.Files) != 2 {
		t.Fatalf("files = %d, want 2", len(body.Files))
	}
	byPath := map[string]int64{}
	for _, f := range body.Files {
		byPath[f.Path] = f.Size
	}
	if byPath["a.txt"] != 5 || byPath["sub/b.txt"] != 2 {
		t.Errorf("sizes wrong: %v", byPath)
	}
}

func TestManifestUploadsFalseWhenNoUpload(t *testing.T) {
	srv, ts := fixture(t, map[string]string{"a.txt": "x"}, false)
	resp := get(t, ts, "/"+srv.Token()+"/manifest.json")
	defer func() { _ = resp.Body.Close() }()
	var body struct {
		Uploads bool `json:"uploads"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body.Uploads {
		t.Error("Uploads should be false with --no-upload")
	}
}

func TestTokenGatingAcrossAllRoutes(t *testing.T) {
	srv, ts := fixture(t, map[string]string{"a.txt": "secret"}, true)
	tok := srv.Token()
	bad := strings.Repeat("0", len(tok))

	routes := []string{"/", "/manifest.json", "/f/a.txt"}
	for _, r := range routes {
		good := get(t, ts, "/"+tok+r)
		_ = good.Body.Close()
		if good.StatusCode == http.StatusNotFound {
			t.Errorf("valid token route %s returned 404", r)
		}

		resp := get(t, ts, "/"+bad+r)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("wrong-token route %s = %d, want 404", r, resp.StatusCode)
		}
	}

	for _, p := range []string{"/", "/" + tok, "/favicon.ico", "/admin", "/" + tok + "x/"} {
		resp := get(t, ts, p)
		_ = resp.Body.Close()
		if p == "/"+tok {
			continue
		}
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("unauthenticated path %s = %d, want 404", p, resp.StatusCode)
		}
	}
}

func TestWrongTokenBodyDoesNotConfirmExistence(t *testing.T) {
	srv, ts := fixture(t, map[string]string{"a.txt": "x"}, true)
	bad := strings.Repeat("f", len(srv.Token()))

	existing := get(t, ts, "/"+bad+"/f/a.txt")
	bodyExisting, _ := io.ReadAll(existing.Body)
	_ = existing.Body.Close()

	missing := get(t, ts, "/"+bad+"/f/nonexistent.txt")
	bodyMissing, _ := io.ReadAll(missing.Body)
	_ = missing.Body.Close()

	if existing.StatusCode != missing.StatusCode {
		t.Errorf("status differs: existing=%d missing=%d", existing.StatusCode, missing.StatusCode)
	}
	if !bytes.Equal(bodyExisting, bodyMissing) {
		t.Errorf("404 body differs between existing and missing file under wrong token")
	}
}

func TestDownloadWrongTokenIsIndistinguishableFromValidTokenMissingFile(t *testing.T) {
	srv, ts := fixture(t, map[string]string{"a.txt": "x"}, true)
	tok := srv.Token()
	bad := strings.Repeat("a", len(tok))

	validMissing := get(t, ts, "/"+tok+"/f/ghost.txt")
	vmBody, _ := io.ReadAll(validMissing.Body)
	_ = validMissing.Body.Close()

	wrongToken := get(t, ts, "/"+bad+"/f/a.txt")
	wtBody, _ := io.ReadAll(wrongToken.Body)
	_ = wrongToken.Body.Close()

	if validMissing.StatusCode != wrongToken.StatusCode {
		t.Errorf("status differs: validMissing=%d wrongToken=%d", validMissing.StatusCode, wrongToken.StatusCode)
	}
	if !bytes.Equal(vmBody, wtBody) {
		t.Errorf("404 bodies differ, leaking token validity")
	}
}

func TestNewTokenIsUnique(t *testing.T) {
	srv1, _ := fixture(t, map[string]string{"a.txt": "x"}, false)
	srv2, _ := fixture(t, map[string]string{"a.txt": "x"}, false)
	if srv1.Token() == srv2.Token() {
		t.Error("two servers produced identical tokens")
	}
	if len(srv1.Token()) != tokenBytes*2 {
		t.Errorf("token length = %d, want %d hex chars", len(srv1.Token()), tokenBytes*2)
	}
}
