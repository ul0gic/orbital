package manifest

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const InboxDir = "sidedrop-inbox"

type Entry struct {
	RelPath string
	AbsPath string
	Size    int64
}

type Manifest struct {
	Root    string
	Entries []Entry
	byPath  map[string]Entry
}

type Options struct {
	AllowAll bool
}

// Lookup returns the entry for a cleaned relative path, or false if the path
// is not a servable file in the manifest.
func (m *Manifest) Lookup(rel string) (Entry, bool) {
	e, ok := m.byPath[rel]
	return e, ok
}

// Build enumerates root once at startup. Only files that survive the exclusion
// rules and symlink-safety checks are servable; nothing else is reachable.
func Build(root string, opts Options) (*Manifest, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolving root %s: %w", root, err)
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return nil, fmt.Errorf("resolving root symlinks %s: %w", absRoot, err)
	}
	info, err := os.Stat(realRoot)
	if err != nil {
		return nil, fmt.Errorf("stat root %s: %w", realRoot, err)
	}
	if info.IsDir() {
		return buildDir(realRoot, opts)
	}
	return buildSingleFile(realRoot)
}

func buildSingleFile(absPath string) (*Manifest, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", absPath, err)
	}
	rel := filepath.Base(absPath)
	entry := Entry{RelPath: rel, AbsPath: absPath, Size: info.Size()}
	return &Manifest{
		Root:    filepath.Dir(absPath),
		Entries: []Entry{entry},
		byPath:  map[string]Entry{rel: entry},
	}, nil
}

func buildDir(realRoot string, opts Options) (*Manifest, error) {
	m := &Manifest{Root: realRoot, byPath: make(map[string]Entry)}
	walkErr := filepath.WalkDir(realRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == realRoot {
			return nil
		}
		rel, relErr := filepath.Rel(realRoot, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if excludedDir(d.Name(), rel, opts) {
				return fs.SkipDir
			}
			return nil
		}
		if entry, ok := evaluateFile(realRoot, path, rel, d, opts); ok {
			m.byPath[rel] = entry
			m.Entries = append(m.Entries, entry)
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("enumerating %s: %w", realRoot, walkErr)
	}
	sort.Slice(m.Entries, func(i, j int) bool {
		return m.Entries[i].RelPath < m.Entries[j].RelPath
	})
	return m, nil
}

func excludedDir(name, rel string, opts Options) bool {
	if name == InboxDir {
		return true
	}
	if opts.AllowAll {
		return false
	}
	return isDotName(name) || matchesDenylist(rel)
}

// evaluateFile returns the servable entry for a file, or ok=false when the file
// is excluded by rule, escapes the root, is unresolvable, or is not a regular
// file. Resolution/stat failures exclude rather than abort: a file removed or
// broken mid-walk is simply not served.
func evaluateFile(realRoot, path, rel string, d fs.DirEntry, opts Options) (entry Entry, ok bool) {
	if !opts.AllowAll && (isDotName(d.Name()) || matchesDenylist(rel)) {
		return Entry{}, false
	}
	resolved, escapes, err := resolveWithinRoot(realRoot, path)
	if err != nil || escapes {
		return Entry{}, false
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		return Entry{}, false
	}
	return Entry{RelPath: rel, AbsPath: resolved, Size: info.Size()}, true
}

// resolveWithinRoot resolves any symlinks and reports whether the target
// escapes realRoot. A non-local relative path or a resolved path outside the
// root is treated as an escape and excluded.
func resolveWithinRoot(realRoot, path string) (resolved string, escapes bool, err error) {
	resolved, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", true, err
	}
	rel, err := filepath.Rel(realRoot, resolved)
	if err != nil {
		return "", true, err
	}
	if !filepath.IsLocal(rel) || strings.HasPrefix(rel, "..") {
		return resolved, true, nil
	}
	return resolved, false, nil
}

func isDotName(name string) bool {
	return strings.HasPrefix(name, ".")
}
