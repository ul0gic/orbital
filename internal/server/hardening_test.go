package server

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ul0gic/orbital/internal/manifest"
)

type httpResult struct {
	status int
	body   []byte
	allow  string
}

func doRead(t *testing.T, method, url string) httpResult {
	t.Helper()
	req, err := http.NewRequest(method, url, http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return httpResult{status: resp.StatusCode, body: body, allow: resp.Header.Get("Allow")}
}

// TestWrongMethodReturnsBare404 is the SEC-004 regression: a matched path with
// the wrong method returns the identical bare 404 as an unknown path — never a
// 405 that confirms the route exists.
func TestWrongMethodReturnsBare404(t *testing.T) {
	srv, ts := fixture(t, map[string]string{"a.txt": "x"}, true)
	tok := srv.Token()

	unknown := doRead(t, http.MethodGet, ts.URL+"/"+tok+"/does-not-exist")
	if unknown.status != http.StatusNotFound {
		t.Fatalf("baseline unknown-path status = %d, want 404", unknown.status)
	}

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/"},
		{http.MethodPost, "/manifest.json"},
		{http.MethodPost, "/f/a.txt"},
		{http.MethodPut, "/manifest.json"},
		{http.MethodGet, "/upload"},
		{http.MethodPut, "/upload"},
		{http.MethodDelete, "/f/a.txt"},
	}
	for _, c := range cases {
		got := doRead(t, c.method, ts.URL+"/"+tok+c.path)
		if got.status != unknown.status {
			t.Errorf("%s %s status = %d, want %d (bare 404)", c.method, c.path, got.status, unknown.status)
		}
		if !bytes.Equal(got.body, unknown.body) {
			t.Errorf("%s %s body = %q, want identical to unknown-path 404 %q", c.method, c.path, got.body, unknown.body)
		}
		if got.allow != "" {
			t.Errorf("%s %s leaked Allow header: %q", c.method, c.path, got.allow)
		}
	}
}

// TestGetUploadDoesNotLeakLanding pins the SEC-004 subtree-match leak fix:
// GET {token}/upload must 404, not serve the landing page.
func TestGetUploadDoesNotLeakLanding(t *testing.T) {
	srv, ts := fixture(t, map[string]string{"a.txt": "x"}, true)
	got := doRead(t, http.MethodGet, ts.URL+"/"+srv.Token()+"/upload")
	if got.status != http.StatusNotFound {
		t.Fatalf("GET /upload status = %d, want 404", got.status)
	}
	if bytes.Contains(got.body, srv.page) || len(got.body) > 100 {
		t.Errorf("GET /upload leaked landing page body (%d bytes)", len(got.body))
	}
}

func TestNoUploadGetUploadIs404(t *testing.T) {
	srv, ts := fixture(t, map[string]string{"a.txt": "x"}, false)
	got := doRead(t, http.MethodGet, ts.URL+"/"+srv.Token()+"/upload")
	if got.status != http.StatusNotFound {
		t.Errorf("--no-upload GET /upload status = %d, want 404", got.status)
	}
}

// TestHardeningHeaders is the SEC-005 regression.
func TestHardeningHeaders(t *testing.T) {
	srv, ts := fixture(t, map[string]string{"a.txt": "x"}, true)
	tok := srv.Token()

	for _, path := range []string{"/", "/manifest.json"} {
		h := headersOf(t, ts.URL+"/"+tok+path)
		if got := h.Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s X-Content-Type-Options = %q, want nosniff", path, got)
		}
		if got := h.Get("Referrer-Policy"); got != "no-referrer" {
			t.Errorf("%s Referrer-Policy = %q, want no-referrer", path, got)
		}
		csp := h.Get("Content-Security-Policy")
		if !strings.Contains(csp, "frame-ancestors 'none'") {
			t.Errorf("%s CSP missing frame-ancestors 'none': %q", path, csp)
		}
		if !strings.Contains(csp, "base-uri 'none'") {
			t.Errorf("%s CSP missing base-uri 'none': %q", path, csp)
		}
	}
}

func headersOf(t *testing.T, url string) http.Header {
	t.Helper()
	resp, err := http.Get(url) //nolint:noctx // test request to local httptest server
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.Header
}

// TestTokenMismatchConstantTimeStillBare404 covers SEC-006: the constant-time
// token comparison keeps the bare-404-on-mismatch behavior.
func TestTokenMismatchConstantTimeStillBare404(t *testing.T) {
	srv, ts := fixture(t, map[string]string{"a.txt": "x"}, true)
	bad := strings.Repeat("0", len(srv.Token()))
	got := doRead(t, http.MethodGet, ts.URL+"/"+bad+"/manifest.json")
	if got.status != http.StatusNotFound {
		t.Fatalf("wrong-token status = %d, want 404", got.status)
	}
	if len(got.body) > 100 {
		t.Errorf("wrong-token 404 body unexpectedly large (%d bytes)", len(got.body))
	}
}

// TestDownloadTraversalCannotEscapeRoot pins the manifest-allowlist property
// behind the semgrep path-traversal finding on handleDownload: rel is only an
// exact map key into the startup enumeration, so no request path can reach a
// file outside the served root.
func TestDownloadTraversalCannotEscapeRoot(t *testing.T) {
	parent := t.TempDir()
	if err := os.WriteFile(filepath.Join(parent, "secret.txt"), []byte("top-secret"), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	root := filepath.Join(parent, "shared")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "pub.txt"), []byte("public"), 0o644); err != nil {
		t.Fatalf("write pub: %v", err)
	}
	m, err := manifest.Build(root, manifest.Options{})
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	srv, err := New(Config{Manifest: m, Events: &recorder{}})
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	ts := httptest.NewServer(srv.routes())
	defer ts.Close()

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	for _, rel := range []string{
		"../secret.txt",
		"%2e%2e/secret.txt",
		"..%2fsecret.txt",
		"pub.txt/../../secret.txt",
	} {
		resp, err := client.Get(ts.URL + "/" + srv.Token() + "/f/" + rel)
		if err != nil {
			t.Fatalf("GET %s: %v", rel, err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Errorf("%s: status 200, traversal must not succeed", rel)
		}
		if bytes.Contains(body, []byte("top-secret")) {
			t.Errorf("%s: response leaked file content outside the served root", rel)
		}
	}
}
