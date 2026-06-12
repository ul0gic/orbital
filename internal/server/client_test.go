package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUAFamily(t *testing.T) {
	cases := map[string]string{
		"":                                    "",
		"Mozilla/5.0 Edg/120.0":               "Edge",
		"Mozilla/5.0 Chrome/120.0":            "Chrome",
		"Mozilla/5.0 Firefox/121.0":           "Firefox",
		"Mozilla/5.0 Version/17 Safari/605.1": "Safari",
		"curl/8.4.0":                          "curl",
		"Wget/1.21":                           "wget",
		"SomeRandomBot/1.0":                   "other",
	}
	for ua, want := range cases {
		if got := uaFamily(ua); got != want {
			t.Errorf("uaFamily(%q) = %q, want %q", ua, got, want)
		}
	}
}

func TestClientHint(t *testing.T) {
	cases := []struct {
		ip   string
		ua   string
		want string
	}{
		{"203.0.113.7", "Mozilla/5.0 Chrome/120.0", "Chrome, 203.0.113.7"},
		{"", "Mozilla/5.0 Chrome/120.0", "Chrome, unknown"},
		{"203.0.113.7", "", "203.0.113.7"},
		{"", "", "unknown"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		if tc.ip != "" {
			req.Header.Set("CF-Connecting-IP", tc.ip)
		}
		if tc.ua != "" {
			req.Header.Set("User-Agent", tc.ua)
		}
		if got := clientHint(req); got != tc.want {
			t.Errorf("clientHint(ip=%q ua=%q) = %q, want %q", tc.ip, tc.ua, got, tc.want)
		}
	}
}

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
