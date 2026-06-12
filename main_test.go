package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ul0gic/orbital/cmd"
	"github.com/ul0gic/orbital/internal/tunnel"
)

// mockTransport is a Transport double for the run-loop test: it records the
// local address run() binds, returns a fixed public URL, and counts Stop calls.
type mockTransport struct {
	mu        sync.Mutex
	publicURL string
	localAddr string
	starts    int
	stops     int
}

func (m *mockTransport) Start(localAddr string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.starts++
	m.localAddr = localAddr
	return m.publicURL, nil
}

func (m *mockTransport) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stops++
	return nil
}

func (m *mockTransport) snapshot() (local string, starts, stops int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.localAddr, m.starts, m.stops
}

// swapTransport installs a transport factory for the duration of the test.
func swapTransport(t *testing.T, mt *mockTransport) {
	t.Helper()
	saved := newTransport
	newTransport = func() tunnel.Transport { return mt }
	t.Cleanup(func() { newTransport = saved })
}

// liveStdout redirects os.Stdout to a pipe and exposes what has been written so
// far while the process is still running. run() writes only the share URL to
// stdout, so a probe goroutine can read the URL the instant ShareReady fires.
type liveStdout struct {
	mu    sync.Mutex
	buf   strings.Builder
	saved *os.File
	w     *os.File
	wg    sync.WaitGroup
}

func captureStdout(t *testing.T) *liveStdout {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	ls := &liveStdout{saved: os.Stdout, w: w}
	os.Stdout = w
	ls.wg.Add(1)
	go func() {
		defer ls.wg.Done()
		chunk := make([]byte, 256)
		for {
			n, readErr := r.Read(chunk)
			if n > 0 {
				ls.mu.Lock()
				ls.buf.Write(chunk[:n])
				ls.mu.Unlock()
			}
			if readErr != nil {
				return
			}
		}
	}()
	return ls
}

func (ls *liveStdout) current() string {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return ls.buf.String()
}

func (ls *liveStdout) restore() string {
	_ = ls.w.Close()
	os.Stdout = ls.saved
	ls.wg.Wait()
	return ls.current()
}

func withArgs(t *testing.T, args []string) {
	t.Helper()
	saved := os.Args
	os.Args = append([]string{"orbital"}, args...)
	t.Cleanup(func() { os.Args = saved })
}

func TestRunLoopServesThenTearsDownOnCancel(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("payload"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	mt := &mockTransport{publicURL: "https://run-loop-test.trycloudflare.com"}
	swapTransport(t, mt)
	withArgs(t, []string{"--no-upload", dir})

	stdout := captureStdout(t)

	ctx, cancel := context.WithCancel(context.Background())

	var (
		probe   serveProbe
		probeWG sync.WaitGroup
	)
	probeWG.Add(1)

	// The wrapper substitutes a cancellable context for Execute's background
	// context, then drives the real run(). A probe hits the live server through
	// the mock's recorded local address before cancelling, simulating Ctrl-C.
	wrapped := func(_ context.Context, cfg cmd.Config, sess *cmd.Session) error {
		go func() {
			defer probeWG.Done()
			probe = pollServer(mt, stdout)
			cancel()
		}()
		return run(ctx, cfg, sess)
	}

	code := cmd.Execute(wrapped)
	probeWG.Wait()
	out := stdout.restore()

	if code != 0 {
		t.Fatalf("Execute exit code = %d, want 0", code)
	}

	_, starts, stops := mt.snapshot()
	if starts != 1 {
		t.Errorf("Transport.Start called %d times, want 1", starts)
	}
	if stops != 1 {
		t.Errorf("Transport.Stop called %d times, want exactly 1", stops)
	}

	if !probe.reached {
		t.Fatal("probe never reached the live server")
	}
	if probe.status != http.StatusOK {
		t.Errorf("download status while up = %d, want 200", probe.status)
	}
	if probe.body != "payload" {
		t.Errorf("downloaded body while up = %q, want payload", probe.body)
	}

	url := strings.TrimSpace(out)
	wantPrefix := mt.publicURL + "/"
	if !strings.HasPrefix(url, wantPrefix) || !strings.HasSuffix(url, "/") {
		t.Fatalf("share URL = %q, want %s{token}/", url, wantPrefix)
	}
	token := strings.TrimSuffix(strings.TrimPrefix(url, wantPrefix), "/")
	if len(token) != 32 {
		t.Errorf("token segment = %q (len %d), want 32 hex chars", token, len(token))
	}
}

type serveProbe struct {
	reached bool
	status  int
	body    string
}

// pollServer waits for the share URL to appear on stdout (ShareReady has fired),
// extracts the token, and downloads the served file through the local address
// the mock recorded — proving the server served while the tunnel was up.
func pollServer(mt *mockTransport, stdout *liveStdout) serveProbe {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		local, _, _ := mt.snapshot()
		token := tokenFromStdout(stdout.current(), mt.publicURL)
		if local == "" || token == "" {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		resp, err := http.Get("http://" + local + "/" + token + "/f/hello.txt")
		if err != nil {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return serveProbe{reached: true, status: resp.StatusCode, body: string(b)}
	}
	return serveProbe{}
}

func tokenFromStdout(out, publicURL string) string {
	line := strings.TrimSpace(out)
	prefix := publicURL + "/"
	if !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, "/") {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(line, prefix), "/")
}
