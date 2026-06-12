package upload

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/ul0gic/orbital/internal/events"
	"github.com/ul0gic/orbital/internal/manifest"
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

func (r *recorder) types() []events.Type {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]events.Type, 0, len(r.events))
	for _, e := range r.events {
		out = append(out, e.Type)
	}
	return out
}

func (r *recorder) has(t events.Type) bool {
	for _, got := range r.types() {
		if got == t {
			return true
		}
	}
	return false
}

func multipartBody(t *testing.T, field, filename string, content []byte) (body *bytes.Buffer, contentType string) {
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
	return &buf, w.FormDataContentType()
}

func postUpload(t *testing.T, h http.Handler, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	body, ct := multipartBody(t, "file", filename, content)
	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func newHandler(t *testing.T, maxBytes int64, pub Publisher) (h *Handler, inbox string) {
	t.Helper()
	root := t.TempDir()
	h, err := New(Config{Root: root, MaxBytes: maxBytes, Events: pub})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h, filepath.Join(root, manifest.InboxDir)
}

func inboxFiles(t *testing.T, inbox string) []string {
	t.Helper()
	entries, err := os.ReadDir(inbox)
	if err != nil {
		t.Fatalf("ReadDir inbox: %v", err)
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

func TestUploadLandsInInbox(t *testing.T) {
	rec := &recorder{}
	h, inbox := newHandler(t, DefaultMaxBytes, rec)

	content := []byte("hello upload world")
	resp := postUpload(t, h, "greeting.txt", content)
	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.Code)
	}

	got := inboxFiles(t, inbox)
	if len(got) != 1 || got[0] != "greeting.txt" {
		t.Fatalf("inbox = %v, want [greeting.txt]", got)
	}
	data, err := os.ReadFile(filepath.Join(inbox, "greeting.txt"))
	if err != nil {
		t.Fatalf("read uploaded file: %v", err)
	}
	if !bytes.Equal(data, content) {
		t.Errorf("content = %q, want %q", data, content)
	}
	if !rec.has(events.UploadStart) || !rec.has(events.UploadComplete) {
		t.Errorf("missing upload events: %v", rec.types())
	}
}

func TestUploadTraversalFilenameNeutralized(t *testing.T) {
	h, inbox := newHandler(t, DefaultMaxBytes, nil)
	resp := postUpload(t, h, "../../escape.txt", []byte("x"))
	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.Code)
	}
	got := inboxFiles(t, inbox)
	if len(got) != 1 || got[0] != "escape.txt" {
		t.Errorf("inbox = %v, want [escape.txt]", got)
	}
	// Nothing must have been written to the parent of the inbox.
	parent := filepath.Dir(inbox)
	if _, err := os.Stat(filepath.Join(parent, "escape.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("file escaped inbox into parent dir")
	}
}

func TestUploadEmptyFilenameRejected(t *testing.T) {
	rec := &recorder{}
	h, inbox := newHandler(t, DefaultMaxBytes, rec)
	resp := postUpload(t, h, "", []byte("x"))
	if resp.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.Code)
	}
	if files := inboxFiles(t, inbox); len(files) != 0 {
		t.Errorf("inbox should be empty, got %v", files)
	}
	if !rec.has(events.UploadRejected) {
		t.Errorf("expected UploadRejected, got %v", rec.types())
	}
}

func TestUploadCollisionGetsNumericSuffix(t *testing.T) {
	h, inbox := newHandler(t, DefaultMaxBytes, nil)

	for i := range 3 {
		resp := postUpload(t, h, "dup.txt", []byte("v"))
		if resp.Code != http.StatusCreated {
			t.Fatalf("upload %d status = %d", i, resp.Code)
		}
	}
	got := inboxFiles(t, inbox)
	want := []string{"dup (1).txt", "dup (2).txt", "dup.txt"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("inbox = %v, want %v", got, want)
	}
}

func TestUploadNeverOverwritesPreexistingFile(t *testing.T) {
	h, inbox := newHandler(t, DefaultMaxBytes, nil)
	preexisting := filepath.Join(inbox, "keep.txt")
	if err := os.WriteFile(preexisting, []byte("original"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	resp := postUpload(t, h, "keep.txt", []byte("new content"))
	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d", resp.Code)
	}
	data, err := os.ReadFile(preexisting)
	if err != nil {
		t.Fatalf("read original: %v", err)
	}
	if string(data) != "original" {
		t.Errorf("preexisting file was overwritten: %q", data)
	}
	if !contains(inboxFiles(t, inbox), "keep (1).txt") {
		t.Errorf("collision copy missing: %v", inboxFiles(t, inbox))
	}
}

func TestUploadOverCapRejectedWith413(t *testing.T) {
	rec := &recorder{}
	h, inbox := newHandler(t, 16, rec)

	resp := postUpload(t, h, "big.bin", bytes.Repeat([]byte("A"), 256))
	if resp.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", resp.Code)
	}
	if files := inboxFiles(t, inbox); len(files) != 0 {
		t.Errorf("over-cap upload left files in inbox: %v", files)
	}
	if !rec.has(events.UploadRejected) {
		t.Errorf("expected UploadRejected event, got %v", rec.types())
	}
}

func TestUploadUnderCapSucceeds(t *testing.T) {
	h, inbox := newHandler(t, 1024, nil)
	resp := postUpload(t, h, "small.bin", bytes.Repeat([]byte("A"), 512))
	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.Code)
	}
	if files := inboxFiles(t, inbox); len(files) != 1 {
		t.Errorf("inbox = %v, want one file", files)
	}
}

func TestUploadNoPartialLeftOnInboxAfterCapHit(t *testing.T) {
	h, inbox := newHandler(t, 16, nil)
	postUpload(t, h, "big.bin", bytes.Repeat([]byte("A"), 256))
	for _, name := range inboxFiles(t, inbox) {
		if strings.HasPrefix(name, ".partial") {
			t.Errorf("leftover .partial temp file: %s", name)
		}
	}
}

func TestUploadNonFilePartRejected(t *testing.T) {
	h, inbox := newHandler(t, DefaultMaxBytes, nil)
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("note", "no file here")
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if files := inboxFiles(t, inbox); len(files) != 0 {
		t.Errorf("inbox should be empty: %v", files)
	}
}

func TestUploadNonMultipartRejected(t *testing.T) {
	h, _ := newHandler(t, DefaultMaxBytes, nil)
	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader("raw body"))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestNewDefaultsMaxBytesWhenNonPositive(t *testing.T) {
	root := t.TempDir()
	h, err := New(Config{Root: root, MaxBytes: 0})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if h.maxBytes != DefaultMaxBytes {
		t.Errorf("maxBytes = %d, want default %d", h.maxBytes, DefaultMaxBytes)
	}
}

func contains(set []string, want string) bool {
	for _, s := range set {
		if s == want {
			return true
		}
	}
	return false
}
