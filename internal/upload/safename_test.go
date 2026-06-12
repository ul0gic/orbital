package upload

import "testing"

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
