package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/ul0gic/orbital/internal/events"
	"github.com/ul0gic/orbital/internal/manifest"
	"github.com/ul0gic/orbital/internal/web"
)

const (
	tokenBytes        = 16
	readHeaderTimeout = 10 * time.Second
	idleTimeout       = 120 * time.Second
)

type Publisher interface {
	Publish(events.Event)
}

type Config struct {
	Manifest *manifest.Manifest
	Events   Publisher
	Upload   http.Handler
}

type Server struct {
	cfg   Config
	token string
	page  []byte
	http  *http.Server
}

// New builds the server with a fresh 128-bit session token. All routes live
// under /{token}/; the token is the path secret. Upload may be nil, in which
// case the upload route is absent and 404s like any other unknown path.
func New(cfg Config) (*Server, error) {
	token, err := newToken()
	if err != nil {
		return nil, fmt.Errorf("generating session token: %w", err)
	}
	page, err := web.Content.ReadFile("index.html")
	if err != nil {
		return nil, fmt.Errorf("loading landing page: %w", err)
	}
	s := &Server{cfg: cfg, token: token, page: page}
	s.http = &http.Server{
		Handler:           s.routes(),
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
	}
	return s, nil
}

// Token is the session path secret; the public URL appends "/{Token}/".
func (s *Server) Token() string { return s.token }

// Serve blocks serving on ln until the server is shut down. It returns nil on a
// clean Shutdown.
func (s *Server) Serve(ln net.Listener) error {
	if err := s.http.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("serving: %w", err)
	}
	return nil
}

// Shutdown gracefully drains in-flight requests, bounded by ctx.
func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.http.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutting down server: %w", err)
	}
	return nil
}

//nolint:gocritic // forwards the published events.Bus.Publish value contract.
func (s *Server) publish(e events.Event) {
	if s.cfg.Events != nil {
		s.cfg.Events.Publish(e)
	}
}

func newToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
