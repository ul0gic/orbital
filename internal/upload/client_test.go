package upload

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUAFamily(t *testing.T) {
	cases := map[string]string{
		"":                          "",
		"Mozilla/5.0 Edg/120.0":     "Edge",
		"Mozilla/5.0 Chrome/120.0":  "Chrome",
		"Mozilla/5.0 Firefox/121.0": "Firefox",
		"Safari/605.1":              "Safari",
		"curl/8.4.0":                "curl",
		"Wget/1.21":                 "wget",
		"WeirdAgent":                "other",
	}
	for ua, want := range cases {
		if got := uaFamily(ua); got != want {
			t.Errorf("uaFamily(%q) = %q, want %q", ua, got, want)
		}
	}
}

func TestClientHint(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/upload", http.NoBody)
	if got := clientHint(req); got != "unknown" {
		t.Errorf("clientHint with no headers = %q, want unknown", got)
	}
	req.Header.Set("CF-Connecting-IP", "198.51.100.4")
	req.Header.Set("User-Agent", "curl/8.4.0")
	if got := clientHint(req); got != "curl, 198.51.100.4" {
		t.Errorf("clientHint = %q, want curl, 198.51.100.4", got)
	}
}
