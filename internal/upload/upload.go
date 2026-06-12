package upload

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/ul0gic/orbital/internal/events"
	"github.com/ul0gic/orbital/internal/manifest"
	"github.com/ul0gic/orbital/internal/visitor"
)

const (
	DefaultMaxBytes int64 = 2 << 30
	// DefaultMaxInbox bounds cumulative accepted bytes across the session.
	DefaultMaxInbox int64 = 8 * DefaultMaxBytes
	// DefaultMaxFiles bounds the number of accepted files across the session.
	DefaultMaxFiles int64 = 1000
	formField             = "file"
)

type Publisher interface {
	Publish(events.Event)
}

type Config struct {
	Root     string
	MaxBytes int64
	MaxInbox int64
	MaxFiles int64
	Events   Publisher
	Visitors *visitor.Registry
}

type Handler struct {
	inbox    string
	maxBytes int64
	maxInbox int64
	maxFiles int64
	events   Publisher
	visitors *visitor.Registry

	usedBytes atomic.Int64
	usedFiles atomic.Int64
}

// New returns the upload handler and creates the inbox directory inside root.
// The inbox lives at root/orbital-inbox; uploads can only ever land here, so
// served files are never overwritten.
func New(cfg Config) (*Handler, error) {
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = DefaultMaxBytes
	}
	if cfg.MaxInbox <= 0 {
		cfg.MaxInbox = DefaultMaxInbox
	}
	if cfg.MaxFiles <= 0 {
		cfg.MaxFiles = DefaultMaxFiles
	}
	if cfg.Visitors == nil {
		cfg.Visitors = visitor.NewRegistry()
	}
	inbox := filepath.Join(cfg.Root, manifest.InboxDir)
	if err := os.MkdirAll(inbox, 0o750); err != nil {
		return nil, fmt.Errorf("creating inbox %s: %w", inbox, err)
	}
	return &Handler{
		inbox:    inbox,
		maxBytes: cfg.MaxBytes,
		maxInbox: cfg.MaxInbox,
		maxFiles: cfg.MaxFiles,
		events:   cfg.Events,
		visitors: cfg.Visitors,
	}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.maxBytes)

	reader, err := r.MultipartReader()
	if err != nil {
		h.reject(w, r, "", maxBytesStatus(err))
		return
	}

	part, err := nextFilePart(reader)
	if err != nil {
		h.reject(w, r, "", maxBytesStatus(err))
		return
	}
	if part == nil {
		h.reject(w, r, "", http.StatusBadRequest)
		return
	}
	defer func() { _ = part.Close() }()

	name, err := safeName(part.FileName())
	if err != nil {
		h.reject(w, r, part.FileName(), http.StatusBadRequest)
		return
	}

	if h.usedFiles.Add(1) > h.maxFiles {
		h.usedFiles.Add(-1)
		h.reject(w, r, name, http.StatusInsufficientStorage)
		return
	}

	client := h.visitors.Hint(r)
	h.publish(events.Event{Type: events.UploadStart, Time: time.Now(), File: name, Client: client})

	final, size, err := h.store(part, name)
	if err != nil {
		h.usedFiles.Add(-1)
		if isTooLarge(err) {
			h.reject(w, r, name, http.StatusRequestEntityTooLarge)
			return
		}
		h.publish(events.Event{Type: events.Error, Time: time.Now(), File: name, Client: client, Err: err.Error()})
		http.Error(w, "upload failed", http.StatusInternalServerError)
		return
	}

	if h.usedBytes.Add(size) > h.maxInbox {
		h.usedBytes.Add(-size)
		h.usedFiles.Add(-1)
		// final is the inbox-confined path claimName just created; removing the
		// over-ceiling file keeps cumulative bytes honest.
		_ = os.Remove(final) //#nosec G703 -- path confined to inbox by construction
		h.reject(w, r, name, http.StatusInsufficientStorage)
		return
	}

	h.publish(events.Event{
		Type:   events.UploadComplete,
		Time:   time.Now(),
		File:   filepath.Base(final),
		Size:   size,
		Bytes:  size,
		Client: client,
	})
	w.WriteHeader(http.StatusCreated)
}

// nextFilePart advances the multipart stream to the first part named "file" and
// returns it unread, so store streams it straight to disk without spooling. It
// returns (nil, nil) when the stream ends with no such part.
func nextFilePart(reader *multipart.Reader) (*multipart.Part, error) {
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if part.FormName() == formField {
			return part, nil
		}
		_ = part.Close()
	}
}

func (h *Handler) reject(w http.ResponseWriter, r *http.Request, name string, status int) {
	h.publish(events.Event{
		Type:   events.UploadRejected,
		Time:   time.Now(),
		File:   name,
		Client: h.visitors.Hint(r),
		Err:    http.StatusText(status),
	})
	http.Error(w, http.StatusText(status), status)
}

//nolint:gocritic // forwards the published events.Bus.Publish value contract.
func (h *Handler) publish(e events.Event) {
	if h.events != nil {
		h.events.Publish(e)
	}
}

func maxBytesStatus(err error) int {
	if isTooLarge(err) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
}

func isTooLarge(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}
