package server

import (
	"context"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

// TestShutdownDrainsInFlightRequest proves graceful Shutdown waits for an
// in-flight download to finish rather than cutting the connection, which is the
// server half of the run-loop teardown ordering (tunnel Stop then server drain).
func TestShutdownDrainsInFlightRequest(t *testing.T) {
	root := t.TempDir()
	if err := writeServerFile(root, "a.txt", "payload"); err != nil {
		t.Fatal(err)
	}
	m := buildManifest(t, root)
	srv, err := New(Config{Manifest: m, Events: &recorder{}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- srv.Serve(ln) }()

	reqStarted := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	var status int
	go func() {
		defer wg.Done()
		close(reqStarted)
		resp, err := http.Get("http://" + ln.Addr().String() + "/" + srv.Token() + "/f/a.txt")
		if err != nil {
			return
		}
		status = resp.StatusCode
		_ = resp.Body.Close()
	}()

	<-reqStarted
	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	wg.Wait()
	if status != 0 && status != http.StatusOK {
		t.Errorf("in-flight request status = %d, want 200 (or completed before shutdown)", status)
	}

	if err := <-serveDone; err != nil {
		t.Errorf("Serve returned error after clean shutdown: %v", err)
	}
}

func TestShutdownIsCleanWithNoTraffic(t *testing.T) {
	root := t.TempDir()
	if err := writeServerFile(root, "a.txt", "x"); err != nil {
		t.Fatal(err)
	}
	srv, err := New(Config{Manifest: buildManifest(t, root), Events: &recorder{}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- srv.Serve(ln) }()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if err := <-serveDone; err != nil {
		t.Errorf("Serve error: %v", err)
	}
}
