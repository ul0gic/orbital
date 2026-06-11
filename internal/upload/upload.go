package upload

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ul0gic/sidedrop/internal/events"
	"github.com/ul0gic/sidedrop/internal/manifest"
)

const (
	DefaultMaxBytes int64 = 2 << 30
	formField             = "file"
)

type Publisher interface {
	Publish(events.Event)
}

type Config struct {
	Root     string
	MaxBytes int64
	Events   Publisher
}

type Handler struct {
	inbox    string
	maxBytes int64
	events   Publisher
}

// New returns the upload handler and creates the inbox directory inside root.
// The inbox lives at root/sidedrop-inbox; uploads can only ever land here, so
// served files are never overwritten.
func New(cfg Config) (*Handler, error) {
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = DefaultMaxBytes
	}
	inbox := filepath.Join(cfg.Root, manifest.InboxDir)
	if err := os.MkdirAll(inbox, 0o750); err != nil {
		return nil, fmt.Errorf("creating inbox %s: %w", inbox, err)
	}
	return &Handler{inbox: inbox, maxBytes: cfg.MaxBytes, events: cfg.Events}, nil
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

	client := clientHint(r)
	h.publish(events.Event{Type: events.UploadStart, Time: time.Now(), File: name, Client: client})

	final, size, err := h.store(part, name)
	if err != nil {
		if isTooLarge(err) {
			h.reject(w, r, name, http.StatusRequestEntityTooLarge)
			return
		}
		h.publish(events.Event{Type: events.Error, Time: time.Now(), File: name, Client: client, Err: err.Error()})
		http.Error(w, "upload failed", http.StatusInternalServerError)
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
		Client: clientHint(r),
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
