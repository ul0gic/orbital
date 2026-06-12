package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile creates a file (and parent dirs) under root with the given relative
// slash-path and content, returning nothing; it fails the test on error.
func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// relPaths returns the sorted relative paths in the manifest for easy assertion.
func relPaths(m *Manifest) []string {
	out := make([]string, 0, len(m.Entries))
	for _, e := range m.Entries {
		out = append(out, e.RelPath)
	}
	return out
}

func contains(set []string, want string) bool {
	for _, s := range set {
		if s == want {
			return true
		}
	}
	return false
}

func TestExclusionDefaultDeny(t *testing.T) {
	denied := []string{
		".bashrc",
		".env",
		".env.local",
		".envrc",
		"id_rsa",
		"id_rsa.pub",
		"server.pem",
		"private.key",
		"credentials",
		"bash_history",
		".ssh/known_hosts",
		".git/config",
		"nested/.git/config",
		"nested/.env",
		"deep/dir/.ssh/id_rsa",
	}
	allowed := []string{
		"readme.txt",
		"photo.jpg",
		"docs/guide.md",
		"environment.txt",
		"keys.txt",
		"history.csv",
	}

	root := t.TempDir()
	for _, p := range append(append([]string{}, denied...), allowed...) {
		writeFile(t, root, p, "x")
	}

	m, err := Build(root, Options{AllowAll: false})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	got := relPaths(m)

	for _, d := range denied {
		if contains(got, d) {
			t.Errorf("denied path leaked into manifest: %s", d)
		}
	}
	for _, a := range allowed {
		if !contains(got, a) {
			t.Errorf("allowed path missing from manifest: %s", a)
		}
	}
}

func TestDenylistGlobEdgeCases(t *testing.T) {
	cases := []struct {
		seg   string
		match bool
	}{
		{".env", true},
		{".env.production", true},
		{".envrc", true},
		{"environment", false},
		{"my.pem", true},
		{"cert.pem.bak", false},
		{"app.key", true},
		{"id_rsa", true},
		{"id_rsa_backup", true},
		{"bash_history", true},
		{"history", true},
		{"historybook", false},
		{"credentials", true},
		{"credentials.json", false},
	}
	for _, tc := range cases {
		if got := matchesDenylist(tc.seg); got != tc.match {
			t.Errorf("matchesDenylist(%q) = %v, want %v", tc.seg, got, tc.match)
		}
	}
}

func TestAllowAllIncludesSensitiveFiles(t *testing.T) {
	root := t.TempDir()
	paths := []string{".env", "id_rsa", "secret.pem", ".ssh/config", "normal.txt"}
	for _, p := range paths {
		writeFile(t, root, p, "x")
	}

	m, err := Build(root, Options{AllowAll: true})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	got := relPaths(m)
	for _, p := range paths {
		if !contains(got, p) {
			t.Errorf("--all should include %s, got %v", p, got)
		}
	}
}

func TestInboxAlwaysExcluded(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, InboxDir+"/leftover.bin", "x")
	writeFile(t, root, "served.txt", "x")

	for _, allowAll := range []bool{false, true} {
		m, err := Build(root, Options{AllowAll: allowAll})
		if err != nil {
			t.Fatalf("Build(allowAll=%v): %v", allowAll, err)
		}
		for _, e := range m.Entries {
			if e.RelPath == InboxDir || strings.HasPrefix(e.RelPath, InboxDir+"/") {
				t.Errorf("inbox file served with allowAll=%v: %s", allowAll, e.RelPath)
			}
		}
	}
}

func TestDotDirectoryExcludedWithContents(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".hidden/secret.txt", "x")
	writeFile(t, root, ".config/app.conf", "x")
	writeFile(t, root, "visible.txt", "x")

	m, err := Build(root, Options{AllowAll: false})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	got := relPaths(m)
	if len(got) != 1 || got[0] != "visible.txt" {
		t.Errorf("dot-directory contents leaked; got %v", got)
	}
}
