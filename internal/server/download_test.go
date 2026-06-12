package server

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/ul0gic/sidedrop/internal/events"
)

func TestDownloadFullByteExact(t *testing.T) {
	content := bytes.Repeat([]byte("sidedrop-"), 4096)
	srv, ts := fixture(t, map[string]string{"big.bin": string(content)}, false)

	resp := get(t, ts, "/"+srv.Token()+"/f/big.bin")
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("downloaded bytes differ from source (got %d, want %d)", len(got), len(content))
	}
}

func TestDownloadNestedPath(t *testing.T) {
	srv, ts := fixture(t, map[string]string{"dir/sub/file.txt": "nested"}, false)
	resp := get(t, ts, "/"+srv.Token()+"/f/dir/sub/file.txt")
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "nested" {
		t.Errorf("body = %q, want nested", body)
	}
}

func TestDownloadRangeReturns206WithCorrectSlice(t *testing.T) {
	content := []byte("0123456789ABCDEFGHIJ")
	srv, ts := fixture(t, map[string]string{"r.txt": string(content)}, false)

	cases := []struct {
		rangeHdr string
		want     string
	}{
		{"bytes=0-4", "01234"},
		{"bytes=5-9", "56789"},
		{"bytes=10-", "ABCDEFGHIJ"},
		{"bytes=-3", "HIJ"},
	}
	for _, tc := range cases {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/"+srv.Token()+"/f/r.txt", http.NoBody)
		req.Header.Set("Range", tc.rangeHdr)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("range %s: %v", tc.rangeHdr, err)
		}
		if resp.StatusCode != http.StatusPartialContent {
			t.Errorf("range %s status = %d, want 206", tc.rangeHdr, resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if string(body) != tc.want {
			t.Errorf("range %s body = %q, want %q", tc.rangeHdr, body, tc.want)
		}
	}
}

func TestDownloadMissingFile404(t *testing.T) {
	srv, ts := fixture(t, map[string]string{"a.txt": "x"}, false)
	resp := get(t, ts, "/"+srv.Token()+"/f/ghost.txt")
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestDownloadEmptyRelPath404(t *testing.T) {
	srv, ts := fixture(t, map[string]string{"a.txt": "x"}, false)
	resp := get(t, ts, "/"+srv.Token()+"/f/")
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestDownloadEmitsStartAndCompleteEvents(t *testing.T) {
	rec := &recorder{}
	root := t.TempDir()
	if err := writeServerFile(root, "a.txt", "payload"); err != nil {
		t.Fatal(err)
	}
	srv, ts := serverFor(t, root, rec, false)
	_ = srv

	resp := get(t, ts, "/"+srv.Token()+"/f/a.txt")
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(body) != "payload" {
		t.Fatalf("body = %q", body)
	}
	if !rec.has(events.DownloadStart) || !rec.has(events.DownloadComplete) {
		t.Errorf("missing download events")
	}
}

func TestLandingEmitsVisitorEvent(t *testing.T) {
	rec := &recorder{}
	root := t.TempDir()
	if err := writeServerFile(root, "a.txt", "x"); err != nil {
		t.Fatal(err)
	}
	srv, ts := serverFor(t, root, rec, false)
	resp := get(t, ts, "/"+srv.Token()+"/")
	_ = resp.Body.Close()
	if !rec.has(events.Visitor) {
		t.Error("landing did not emit Visitor event")
	}
}

func TestClientHintFromCloudflareHeader(t *testing.T) {
	srv, ts := fixture(t, map[string]string{"a.txt": "x"}, false)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/"+srv.Token()+"/", http.NoBody)
	req.Header.Set("CF-Connecting-IP", "203.0.113.7")
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d", resp.StatusCode)
	}
}

func TestNoUploadRouteRejectsPost(t *testing.T) {
	// ISSUE-001: with --no-upload (Upload nil) the upload route is unregistered,
	// but a POST to {token}/upload currently returns 405 (matched by the GET-only
	// subtree pattern) rather than the documented no-confirm 404. This test pins
	// the CURRENT behavior; flip the expectation when ISSUE-001 is fixed.
	srv, ts := fixture(t, map[string]string{"a.txt": "x"}, false)
	resp, err := http.Post(ts.URL+"/"+srv.Token()+"/upload", "text/plain", nil)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405 (documented in ISSUE-001; should become 404)", resp.StatusCode)
	}
}
