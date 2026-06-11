package tunnel

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	readyTimeout  = 30 * time.Second
	stopGrace     = 5 * time.Second
	registeredLog = "Registered tunnel connection"
)

var urlPattern = regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)

var ErrTunnelTimeout = errors.New("cloudflared did not become ready within timeout")

type Cloudflared struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	stopped bool
}

var _ Transport = (*Cloudflared)(nil)

func NewCloudflared() *Cloudflared { return &Cloudflared{} }

// Start launches cloudflared against localAddr in its own process group and
// blocks until both the public URL and a registered-connection line appear, or
// the readiness timeout elapses. On any failure the process group is torn down.
func (c *Cloudflared) Start(localAddr string) (string, error) {
	if _, err := exec.LookPath("cloudflared"); err != nil {
		return "", &ErrCloudflaredNotFound{GOOS: runtime.GOOS}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// The tunnel outlives any request: its lifetime is owned by Stop() and the
	// process group, so a non-cancellable background context is intentional.
	// Fixed argv, no shell; localAddr is our own listener address (127.0.0.1:port).
	cmd := exec.CommandContext(context.Background(), "cloudflared", "tunnel", "--url", "http://"+localAddr, "--no-autoupdate") //#nosec G204 -- constant argv; localAddr is internally produced
	setProcessGroup(cmd)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("capturing cloudflared stderr: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("starting cloudflared: %w", err)
	}
	c.cmd = cmd

	url, err := awaitReady(stderr, readyTimeout)
	if err != nil {
		_ = c.terminate()
		return "", err
	}
	return url, nil
}

// Stop tears down the entire cloudflared process group. It is safe to call from
// defer or signal paths and idempotent across repeated calls.
func (c *Cloudflared) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.terminate()
}

func (c *Cloudflared) terminate() error {
	if c.cmd == nil || c.cmd.Process == nil || c.stopped {
		c.stopped = true
		return nil
	}
	c.stopped = true
	return killGroup(c.cmd, stopGrace)
}

type readyResult struct {
	url string
	err error
}

func awaitReady(stderr io.Reader, timeout time.Duration) (string, error) {
	done := make(chan readyResult, 1)
	go scanReady(stderr, done)

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case res := <-done:
		return res.url, res.err
	case <-timer.C:
		return "", ErrTunnelTimeout
	}
}

// scanReady reads cloudflared stderr until both the public URL and the
// registered-connection line are seen, then reports the URL. It drains stderr
// to EOF on failure so the pipe never blocks the child.
func scanReady(stderr io.Reader, done chan<- readyResult) {
	scanner := bufio.NewScanner(stderr)
	var url string
	var registered bool
	for scanner.Scan() {
		line := scanner.Text()
		if url == "" {
			if m := urlPattern.FindString(line); m != "" {
				url = m
			}
		}
		if !registered && strings.Contains(line, registeredLog) {
			registered = true
		}
		if url != "" && registered {
			done <- readyResult{url: url}
			drain(stderr)
			return
		}
	}
	if err := scanner.Err(); err != nil {
		done <- readyResult{err: fmt.Errorf("reading cloudflared output: %w", err)}
		return
	}
	done <- readyResult{err: errors.New("cloudflared exited before becoming ready")}
}

func drain(r io.Reader) {
	_, _ = io.Copy(io.Discard, r)
}
