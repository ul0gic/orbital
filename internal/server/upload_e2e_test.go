package server

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ul0gic/sidedrop/internal/events"
	"github.com/ul0gic/sidedrop/internal/manifest"
)

func postFile(t *testing.T, url, field, filename string, content []byte) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile(field, filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	resp, err := http.Post(url, w.FormDataContentType(), &buf)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	return resp
}

func inbox(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, manifest.InboxDir))
	if err != nil {
		t.Fatalf("read inbox: %v", err)
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

func TestUploadE2ELandsInInbox(t *testing.T) {
	rec := &recorder{}
	root := t.TempDir()
	if err := writeServerFile(root, "served.txt", "x"); err != nil {
		t.Fatal(err)
	}
	srv, ts := serverFor(t, root, rec, true)

	content := []byte("uploaded payload bytes")
	resp := postFile(t, ts.URL+"/"+srv.Token()+"/upload", "file", "doc.txt", content)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	got := inbox(t, root)
	if len(got) != 1 || got[0] != "doc.txt" {
		t.Fatalf("inbox = %v, want [doc.txt]", got)
	}
	data, err := os.ReadFile(filepath.Join(root, manifest.InboxDir, "doc.txt"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(data, content) {
		t.Errorf("content mismatch")
	}
	if !rec.has(events.UploadComplete) {
		t.Error("missing UploadComplete event")
	}
}

func TestUploadE2ECollisionThroughServer(t *testing.T) {
	root := t.TempDir()
	if err := writeServerFile(root, "served.txt", "x"); err != nil {
		t.Fatal(err)
	}
	srv, ts := serverFor(t, root, &recorder{}, true)
	url := ts.URL + "/" + srv.Token() + "/upload"

	for i := range 2 {
		resp := postFile(t, url, "file", "same.txt", []byte("v"))
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("upload %d = %d", i, resp.StatusCode)
		}
	}
	got := inbox(t, root)
	if len(got) != 2 || got[0] != "same (1).txt" || got[1] != "same.txt" {
		t.Errorf("inbox = %v, want [same (1).txt same.txt]", got)
	}
}

func TestUploadE2ERequiresToken(t *testing.T) {
	root := t.TempDir()
	if err := writeServerFile(root, "served.txt", "x"); err != nil {
		t.Fatal(err)
	}
	srv, ts := serverFor(t, root, &recorder{}, true)
	bad := strings.Repeat("0", len(srv.Token()))

	resp := postFile(t, ts.URL+"/"+bad+"/upload", "file", "x.txt", []byte("x"))
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("wrong-token upload = %d, want 404", resp.StatusCode)
	}
	if entries, _ := os.ReadDir(filepath.Join(root, manifest.InboxDir)); len(entries) != 0 {
		t.Errorf("file landed despite wrong token")
	}
}
