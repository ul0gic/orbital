package tunnel

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ul0gic/sidedrop/internal/manifest"
	"github.com/ul0gic/sidedrop/internal/server"
)

// TestTeardownOrderStopsTransportOnceThenDrains mirrors main.run's teardown:
// the public link (Transport.Stop) dies first, then the local server drains.
// It asserts Stop is called exactly once and the server shuts down cleanly,
// without spawning a real cloudflared.
func TestTeardownOrderStopsTransportOnceThenDrains(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	m, err := manifest.Build(root, manifest.Options{})
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	srv, err := server.New(server.Config{Manifest: m})
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- srv.Serve(ln) }()

	mt := newMockTransport("https://teardown-test.trycloudflare.com")
	url, err := mt.Start(ln.Addr().String())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if url == "" {
		t.Fatal("Start returned empty url")
	}

	stopErr := mt.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	shutErr := srv.Shutdown(ctx)

	if err := errors.Join(stopErr, shutErr); err != nil {
		t.Fatalf("teardown error: %v", err)
	}
	if mt.stopCount() != 1 {
		t.Errorf("Stop called %d times, want exactly 1", mt.stopCount())
	}
	if mt.localAddr() != ln.Addr().String() {
		t.Errorf("Transport started against %q, want listener addr %q", mt.localAddr(), ln.Addr().String())
	}
	if err := <-serveDone; err != nil {
		t.Errorf("Serve returned error after clean shutdown: %v", err)
	}
}
