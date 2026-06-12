package server

import "testing"

func TestDownloadRel(t *testing.T) {
	tok := "abc123"
	cases := []struct {
		path string
		want string
	}{
		{"/abc123/f/file.txt", "file.txt"},
		{"/abc123/f/sub/file.txt", "sub/file.txt"},
		{"/abc123/f/", ""},
		{"/abc123/manifest.json", ""},
		{"/wrong/f/file.txt", ""},
	}
	for _, tc := range cases {
		if got := downloadRel(tc.path, tok); got != tc.want {
			t.Errorf("downloadRel(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}
