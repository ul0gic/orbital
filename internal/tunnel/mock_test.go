package tunnel

import (
	"errors"
	"sync"
	"testing"
)

// mockTransport is a test double for the Transport interface. It records every
// Start/Stop call, returns a configurable fake URL, and can be wired to fail on
// demand. It is safe for concurrent use so lifecycle and integration tests can
// drive it from multiple goroutines.
type mockTransport struct {
	mu sync.Mutex

	url       string
	startErr  error
	stopErr   error
	starts    int
	stops     int
	lastLocal string
}

var _ Transport = (*mockTransport)(nil)

func newMockTransport(url string) *mockTransport {
	return &mockTransport{url: url}
}

func (m *mockTransport) Start(localAddr string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.starts++
	m.lastLocal = localAddr
	if m.startErr != nil {
		return "", m.startErr
	}
	return m.url, nil
}

func (m *mockTransport) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stops++
	return m.stopErr
}

func (m *mockTransport) startCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.starts
}

func (m *mockTransport) stopCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stops
}

func (m *mockTransport) localAddr() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastLocal
}

func TestMockTransportRecordsStartAndReturnsURL(t *testing.T) {
	mt := newMockTransport("https://example.trycloudflare.com")
	url, err := mt.Start("127.0.0.1:8080")
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if url != "https://example.trycloudflare.com" {
		t.Errorf("url = %q, want fake url", url)
	}
	if mt.startCount() != 1 {
		t.Errorf("starts = %d, want 1", mt.startCount())
	}
	if mt.localAddr() != "127.0.0.1:8080" {
		t.Errorf("localAddr = %q, want recorded local address", mt.localAddr())
	}
}

func TestMockTransportStartFailureMode(t *testing.T) {
	want := errors.New("boom")
	mt := newMockTransport("")
	mt.startErr = want
	if _, err := mt.Start("127.0.0.1:0"); !errors.Is(err, want) {
		t.Errorf("Start err = %v, want %v", err, want)
	}
}

func TestMockTransportStopRecordsAndCanFail(t *testing.T) {
	mt := newMockTransport("")
	if err := mt.Stop(); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
	if mt.stopCount() != 1 {
		t.Errorf("stops = %d, want 1", mt.stopCount())
	}
	mt.stopErr = errors.New("stop failed")
	if err := mt.Stop(); err == nil {
		t.Error("Stop did not surface configured error")
	}
	if mt.stopCount() != 2 {
		t.Errorf("stops = %d, want 2", mt.stopCount())
	}
}
