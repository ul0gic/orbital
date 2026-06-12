package visitor

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

func req(ip, ua string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	if ip != "" {
		r.Header.Set("CF-Connecting-IP", ip)
	}
	if ua != "" {
		r.Header.Set("User-Agent", ua)
	}
	return r
}

func TestHintNeverContainsAddress(t *testing.T) {
	g := NewRegistry()
	cases := []struct {
		ip   string
		ua   string
		want string
	}{
		{"203.0.113.7", "Mozilla/5.0 Chrome/120.0", "Chrome, visitor 1"},
		{"203.0.113.7", "", "visitor 1"},
		{"2001:db8::4f", "Mozilla/5.0 Version/17 Safari/605.1", "Safari, visitor 2"},
		{"", "Mozilla/5.0 Chrome/120.0", "Chrome, visitor 3"},
		{"", "", "visitor 3"},
	}
	for _, tc := range cases {
		if got := g.Hint(req(tc.ip, tc.ua)); got != tc.want {
			t.Errorf("Hint(ip=%q ua=%q) = %q, want %q", tc.ip, tc.ua, got, tc.want)
		}
	}
}

func TestNumbersAreStablePerAddress(t *testing.T) {
	g := NewRegistry()
	first := g.Hint(req("198.51.100.4", ""))
	for range 3 {
		if got := g.Hint(req("198.51.100.4", "")); got != first {
			t.Errorf("repeat visit = %q, want %q", got, first)
		}
	}
	if got := g.Hint(req("198.51.100.5", "")); got == first {
		t.Errorf("distinct address got same tag %q", got)
	}
}
