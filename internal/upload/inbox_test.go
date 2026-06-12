package upload

import (
	"net/http"
	"testing"

	"github.com/ul0gic/orbital/internal/events"
	"github.com/ul0gic/orbital/internal/manifest"
)

func newBoundedHandler(t *testing.T, maxBytes, maxInbox, maxFiles int64, pub Publisher) (h *Handler, inbox string) {
	t.Helper()
	root := t.TempDir()
	h, err := New(Config{
		Root:     root,
		MaxBytes: maxBytes,
		MaxInbox: maxInbox,
		MaxFiles: maxFiles,
		Events:   pub,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h, joinInbox(root)
}

func joinInbox(root string) string {
	return root + "/" + manifest.InboxDir
}

func TestUploadAcceptedUnderInboxByteCeiling(t *testing.T) {
	rec := &recorder{}
	h, inbox := newBoundedHandler(t, DefaultMaxBytes, 100, 100, rec)

	resp := postUpload(t, h, "a.txt", []byte("0123456789")) // 10 bytes
	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.Code)
	}
	if got := inboxFiles(t, inbox); len(got) != 1 {
		t.Fatalf("inbox files = %v, want 1", got)
	}
}

func TestUploadRejectedOverInboxByteCeiling(t *testing.T) {
	rec := &recorder{}
	h, inbox := newBoundedHandler(t, DefaultMaxBytes, 15, 100, rec)

	first := postUpload(t, h, "a.txt", []byte("0123456789")) // 10 bytes, under 15
	if first.Code != http.StatusCreated {
		t.Fatalf("first status = %d, want 201", first.Code)
	}

	second := postUpload(t, h, "b.txt", []byte("0123456789")) // would push to 20 > 15
	if second.Code != http.StatusInsufficientStorage {
		t.Fatalf("second status = %d, want 507", second.Code)
	}
	if !rec.has(events.UploadRejected) {
		t.Errorf("expected UploadRejected event, got %v", rec.types())
	}
	if got := inboxFiles(t, inbox); len(got) != 1 {
		t.Errorf("over-ceiling upload was written: inbox = %v, want only the first file", got)
	}
}

func TestUploadRejectedOverFileCountCeiling(t *testing.T) {
	rec := &recorder{}
	h, inbox := newBoundedHandler(t, DefaultMaxBytes, DefaultMaxInbox, 1, rec)

	if first := postUpload(t, h, "a.txt", []byte("x")); first.Code != http.StatusCreated {
		t.Fatalf("first status = %d, want 201", first.Code)
	}
	second := postUpload(t, h, "b.txt", []byte("y"))
	if second.Code != http.StatusInsufficientStorage {
		t.Fatalf("second status = %d, want 507", second.Code)
	}
	if got := inboxFiles(t, inbox); len(got) != 1 {
		t.Errorf("count-ceiling breach wrote a file: inbox = %v", got)
	}
}

func TestInboxDefaultsApplied(t *testing.T) {
	h, err := New(Config{Root: t.TempDir()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if h.maxInbox != DefaultMaxInbox {
		t.Errorf("maxInbox = %d, want default %d", h.maxInbox, DefaultMaxInbox)
	}
	if h.maxFiles != DefaultMaxFiles {
		t.Errorf("maxFiles = %d, want default %d", h.maxFiles, DefaultMaxFiles)
	}
}
