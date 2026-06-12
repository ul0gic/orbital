package manifest

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestBuildEnumeratesSizesAndRelPaths(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "a.txt", "hello")
	writeFile(t, root, "sub/b.txt", "world!!")
	writeFile(t, root, "sub/deep/c.bin", "xyz")

	m, err := Build(root, Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	want := map[string]int64{
		"a.txt":          5,
		"sub/b.txt":      7,
		"sub/deep/c.bin": 3,
	}
	if len(m.Entries) != len(want) {
		t.Fatalf("entry count = %d, want %d (%v)", len(m.Entries), len(want), relPaths(m))
	}
	for _, e := range m.Entries {
		size, ok := want[e.RelPath]
		if !ok {
			t.Errorf("unexpected entry %s", e.RelPath)
			continue
		}
		if e.Size != size {
			t.Errorf("%s size = %d, want %d", e.RelPath, e.Size, size)
		}
		if !filepath.IsAbs(e.AbsPath) {
			t.Errorf("%s AbsPath not absolute: %s", e.RelPath, e.AbsPath)
		}
	}
}

func TestBuildEntriesSorted(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"zebra.txt", "alpha.txt", "mango.txt", "sub/beta.txt"} {
		writeFile(t, root, p, "x")
	}
	m, err := Build(root, Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	got := relPaths(m)
	if !sort.StringsAreSorted(got) {
		t.Errorf("entries not sorted: %v", got)
	}
}

func TestBuildSingleFileMode(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "report.pdf", "PDF-CONTENT")
	target := filepath.Join(root, "report.pdf")

	m, err := Build(target, Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(m.Entries) != 1 {
		t.Fatalf("single-file mode entries = %d, want 1", len(m.Entries))
	}
	e := m.Entries[0]
	if e.RelPath != "report.pdf" {
		t.Errorf("RelPath = %q, want report.pdf", e.RelPath)
	}
	if e.Size != int64(len("PDF-CONTENT")) {
		t.Errorf("Size = %d, want %d", e.Size, len("PDF-CONTENT"))
	}
	if m.Root != root {
		t.Errorf("Root = %q, want %q", m.Root, root)
	}
}

func TestBuildEmptyDir(t *testing.T) {
	root := t.TempDir()
	m, err := Build(root, Options{})
	if err != nil {
		t.Fatalf("Build empty dir: %v", err)
	}
	if len(m.Entries) != 0 {
		t.Errorf("empty dir entries = %d, want 0", len(m.Entries))
	}
}

func TestBuildDirWithOnlyExcludedFilesIsEmpty(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".env", "x")
	writeFile(t, root, "id_rsa", "x")

	m, err := Build(root, Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(m.Entries) != 0 {
		t.Errorf("entries = %d, want 0 (%v)", len(m.Entries), relPaths(m))
	}
}

func TestBuildMissingRootErrors(t *testing.T) {
	_, err := Build(filepath.Join(t.TempDir(), "does-not-exist"), Options{})
	if err == nil {
		t.Fatal("expected error for missing root")
	}
}

func TestLookupHitsAndMisses(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "a.txt", "data")
	writeFile(t, root, "sub/b.txt", "more")

	m, err := Build(root, Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if e, ok := m.Lookup("a.txt"); !ok || e.RelPath != "a.txt" {
		t.Errorf("Lookup(a.txt) ok=%v entry=%+v", ok, e)
	}
	if e, ok := m.Lookup("sub/b.txt"); !ok || e.Size != 4 {
		t.Errorf("Lookup(sub/b.txt) ok=%v entry=%+v", ok, e)
	}
	for _, miss := range []string{"missing.txt", "sub", "../a.txt", "sub/../a.txt", ""} {
		if _, ok := m.Lookup(miss); ok {
			t.Errorf("Lookup(%q) unexpectedly hit", miss)
		}
	}
}

func TestSymlinkToOutsideRootExcluded(t *testing.T) {
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("top secret"), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	root := t.TempDir()
	writeFile(t, root, "normal.txt", "ok")
	link := filepath.Join(root, "leak.txt")
	if err := os.Symlink(secret, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	m, err := Build(root, Options{AllowAll: true})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	got := relPaths(m)
	if contains(got, "leak.txt") {
		t.Errorf("symlink escaping root was served: %v", got)
	}
	if !contains(got, "normal.txt") {
		t.Errorf("normal file missing: %v", got)
	}
}

func TestSymlinkWithinRootResolvingOutsideViaParentExcluded(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "sub/real.txt", "in root")
	// Symlink target uses .. to climb above root entirely.
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "x.txt"), []byte("out"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	link := filepath.Join(root, "sub", "escape.txt")
	rel, err := filepath.Rel(filepath.Join(root, "sub"), filepath.Join(outside, "x.txt"))
	if err != nil {
		t.Fatalf("rel: %v", err)
	}
	if err := os.Symlink(rel, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	m, err := Build(root, Options{AllowAll: true})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if contains(relPaths(m), "sub/escape.txt") {
		t.Errorf("relative ..-escaping symlink served: %v", relPaths(m))
	}
}

func TestSymlinkWithinRootIsAllowed(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "data/target.txt", "linked content")
	link := filepath.Join(root, "alias.txt")
	if err := os.Symlink(filepath.Join(root, "data", "target.txt"), link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	m, err := Build(root, Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	got := relPaths(m)
	if !contains(got, "alias.txt") {
		t.Errorf("in-root symlink should be served: %v", got)
	}
	if e, ok := m.Lookup("alias.txt"); ok && e.Size != int64(len("linked content")) {
		t.Errorf("alias size = %d, want %d", e.Size, len("linked content"))
	}
}
