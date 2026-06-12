package tunnel

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestAwaitReadyExtractsURLWhenRegistered(t *testing.T) {
	out := strings.Join([]string{
		"some startup noise",
		"failed to sufficiently increase receive buffer size",
		"|  https://crossing-shareware-foo-bar.trycloudflare.com    |",
		"INF Registered tunnel connection connIndex=0",
		"trailing noise after ready",
	}, "\n")

	url, err := awaitReady(strings.NewReader(out), time.Second)
	if err != nil {
		t.Fatalf("awaitReady error: %v", err)
	}
	want := "https://crossing-shareware-foo-bar.trycloudflare.com"
	if url != want {
		t.Errorf("url = %q, want %q", url, want)
	}
}

func TestAwaitReadyWaitsForBothURLAndRegistration(t *testing.T) {
	// URL present but never registered: must not report ready, must surface the
	// exited-before-ready error once the stream ends.
	out := "https://only-url-no-registration.trycloudflare.com\nmore lines\n"
	_, err := awaitReady(strings.NewReader(out), time.Second)
	if err == nil {
		t.Fatal("expected error when registration line never appears")
	}
	if strings.Contains(err.Error(), "trycloudflare") {
		t.Errorf("error should not leak URL: %v", err)
	}
}

func TestAwaitReadyTimeout(t *testing.T) {
	pr, pw := io.Pipe()
	defer func() { _ = pw.Close() }()

	start := time.Now()
	_, err := awaitReady(pr, 20*time.Millisecond)
	if !errors.Is(err, ErrTunnelTimeout) {
		t.Errorf("err = %v, want ErrTunnelTimeout", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("timeout took too long: %v", elapsed)
	}
}

func TestAwaitReadyExitedBeforeReady(t *testing.T) {
	_, err := awaitReady(strings.NewReader("partial line, no url, then EOF\n"), time.Second)
	if err == nil {
		t.Fatal("expected error on premature EOF")
	}
}

func TestURLPatternMatchesQuickTunnelOnly(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{"https://crossing-shareware.trycloudflare.com", "https://crossing-shareware.trycloudflare.com"},
		{"| https://a-b-c-d.trycloudflare.com |", "https://a-b-c-d.trycloudflare.com"},
		{"https://evil.com/?x=trycloudflare.com", ""},
		{"http://no-https.trycloudflare.com", ""},
		{"nothing here", ""},
	}
	for _, tc := range cases {
		if got := urlPattern.FindString(tc.line); got != tc.want {
			t.Errorf("FindString(%q) = %q, want %q", tc.line, got, tc.want)
		}
	}
}

func TestErrCloudflaredNotFoundInstallHint(t *testing.T) {
	cases := map[string]string{
		"darwin":  "brew install cloudflared",
		"linux":   "cloudflared-linux-amd64",
		"windows": "developers.cloudflare.com",
	}
	for goos, substr := range cases {
		err := &ErrCloudflaredNotFound{GOOS: goos}
		if err.Error() != "cloudflared not found on PATH" {
			t.Errorf("Error() = %q", err.Error())
		}
		if !strings.Contains(err.InstallHint(), substr) {
			t.Errorf("InstallHint(%s) = %q, want substring %q", goos, err.InstallHint(), substr)
		}
	}
}
