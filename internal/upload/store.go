package upload

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxCollisionAttempts = 10000
	maxNameBytes         = 200
)

var (
	errEmptyName   = errors.New("empty filename")
	errNameTooLong = errors.New("filename too long")
	errNameControl = errors.New("filename contains control characters")
)

// safeName reduces an uploaded filename to a single safe basename. Path
// separators and traversal are stripped; the result is guaranteed to be a plain
// name that cannot escape the inbox. Control characters are rejected so the
// name never reaches disk or the live log, and the length is capped so an
// over-long name fails as a 400 rather than an ENAMETOOLONG 500.
func safeName(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.ReplaceAll(raw, "\\", "/")
	base := filepath.Base(filepath.FromSlash(raw))
	if base == "." || base == ".." || base == string(filepath.Separator) || base == "" {
		return "", errEmptyName
	}
	if !filepath.IsLocal(base) {
		return "", fmt.Errorf("unsafe filename %q", raw)
	}
	if strings.ContainsFunc(base, isControl) {
		return "", errNameControl
	}
	if len(base) > maxNameBytes {
		return "", errNameTooLong
	}
	return base, nil
}

func isControl(r rune) bool {
	return r < 0x20 || r == 0x7f
}

// store streams src to a temp file in the inbox, then atomically renames it to a
// collision-free final name. The temp file is removed on any failure so a
// partial upload never appears in the inbox.
func (h *Handler) store(src io.Reader, name string) (finalPath string, size int64, err error) {
	tmp, err := os.CreateTemp(h.inbox, ".partial-*")
	if err != nil {
		return "", 0, fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	size, err = io.Copy(tmp, src)
	if err != nil {
		_ = tmp.Close()
		return "", 0, err
	}
	if err = tmp.Close(); err != nil {
		return "", 0, fmt.Errorf("closing temp file: %w", err)
	}

	final, err := h.claimName(name)
	if err != nil {
		return "", 0, err
	}
	if err = os.Rename(tmpName, final); err != nil {
		return "", 0, fmt.Errorf("finalizing upload: %w", err)
	}
	cleanup = false
	return final, size, nil
}

// claimName reserves a non-colliding path in the inbox by creating an empty
// placeholder: name.ext, then "name (1).ext", "name (2).ext", … The placeholder
// is created O_EXCL so concurrent uploads cannot claim the same name.
func (h *Handler) claimName(name string) (string, error) {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 0; i <= maxCollisionAttempts; i++ {
		candidate := name
		if i > 0 {
			candidate = fmt.Sprintf("%s (%d)%s", stem, i, ext)
		}
		full := filepath.Join(h.inbox, candidate)
		// candidate is a sanitized basename (safeName: IsLocal + Base), joined to
		// the trusted inbox and cleaned by Join — it cannot escape the inbox.
		f, err := os.OpenFile(full, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //#nosec G304 G703 -- path confined to inbox by construction
		if err == nil {
			_ = f.Close()
			return full, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", fmt.Errorf("reserving name: %w", err)
		}
	}
	return "", fmt.Errorf("too many collisions for %q", name)
}
