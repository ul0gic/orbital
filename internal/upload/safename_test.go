package upload

import (
	"strings"
	"testing"
)

func TestSafeNameNeutralizesTraversal(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"photo.jpg", "photo.jpg"},
		{"../x.txt", "x.txt"},
		{"../../etc/passwd", "passwd"},
		{"..\\..\\windows\\system32\\cfg", "cfg"},
		{"/etc/shadow", "shadow"},
		{"  spaced.png  ", "spaced.png"},
		{"sub/dir/file.bin", "file.bin"},
		{"a/b/../c.txt", "c.txt"},
	}
	for _, tc := range cases {
		got, err := safeName(tc.raw)
		if err != nil {
			t.Errorf("safeName(%q) error: %v", tc.raw, err)
			continue
		}
		if got != tc.want {
			t.Errorf("safeName(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestSafeNameRejectsEmptyAndDotForms(t *testing.T) {
	for _, raw := range []string{"", "   ", ".", "..", "../", "/", "\\", "../..", "./"} {
		if got, err := safeName(raw); err == nil {
			t.Errorf("safeName(%q) = %q, want error", raw, got)
		}
	}
}

func TestSafeNameRejectsControlChars(t *testing.T) {
	for _, raw := range []string{
		"a\x1bb.txt",   // ESC
		"\x00null.txt", // NUL
		"bell\a.txt",   // BEL
		"cr\r.txt",     // CR
		"tab\t.txt",    // TAB
		"del\x7f.txt",  // DEL
	} {
		if got, err := safeName(raw); err == nil {
			t.Errorf("safeName(%q) = %q, want error for control char", raw, got)
		}
	}
}

func TestSafeNameRejectsOverLongName(t *testing.T) {
	long := strings.Repeat("a", maxNameBytes+1) + ".txt"
	if got, err := safeName(long); err == nil {
		t.Errorf("safeName(<%d bytes>) = %q, want error", len(long), got)
	}
	ok := strings.Repeat("a", maxNameBytes-4) + ".txt"
	if _, err := safeName(ok); err != nil {
		t.Errorf("safeName(<=%d bytes) error: %v", maxNameBytes, err)
	}
}
