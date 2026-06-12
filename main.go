package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ul0gic/sidedrop/cmd"
	"github.com/ul0gic/sidedrop/internal/events"
	"github.com/ul0gic/sidedrop/internal/manifest"
	"github.com/ul0gic/sidedrop/internal/server"
	"github.com/ul0gic/sidedrop/internal/tunnel"
	"github.com/ul0gic/sidedrop/internal/upload"
)

const shutdownGrace = 10 * time.Second

func main() {
	os.Exit(cmd.Execute(run))
}

func run(ctx context.Context, cfg cmd.Config, sess *cmd.Session) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	m, err := manifest.Build(cfg.Path, manifest.Options{AllowAll: cfg.All})
	if err != nil {
		return err
	}
	if len(m.Entries) == 0 {
		return fmt.Errorf("nothing to serve in %s (hidden and sensitive files are excluded; --all overrides)", cfg.Path)
	}

	bus := events.NewBus()

	var uploadHandler http.Handler
	if !cfg.NoUpload {
		h, err := upload.New(upload.Config{Root: m.Root, MaxBytes: cfg.MaxUpload, Events: bus})
		if err != nil {
			return err
		}
		uploadHandler = h
	}

	srv, err := server.New(server.Config{Manifest: m, Events: bus, Upload: uploadHandler})
	if err != nil {
		return err
	}

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", cfg.Port))
	if err != nil {
		return fmt.Errorf("binding local port: %w", err)
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	transport := tunnel.NewCloudflared()
	//nolint:contextcheck // Transport is a frozen contract without a ctx parameter; Start enforces its own 30s readiness timeout.
	publicURL, err := transport.Start(ln.Addr().String())
	if err != nil {
		return errors.Join(err, shutdown(ctx, srv))
	}

	stream, cancelLog := bus.Subscribe()
	defer cancelLog()
	logDone := make(chan struct{})
	go func() {
		sess.UI.Log(stream)
		close(logDone)
	}()

	shareURL := publicURL + "/" + srv.Token() + "/"
	sess.ShareReady(shareURL, len(m.Entries))
	bus.Publish(events.Event{Type: events.Ready, Time: time.Now(), File: shareURL})

	var runErr error
	select {
	case <-ctx.Done():
	case err := <-serveErr:
		runErr = err
	}

	// Tunnel first: the public link must die the moment the session ends,
	// before the local server finishes draining.
	stopErr := transport.Stop()
	shutErr := shutdown(ctx, srv)

	cancelLog()
	select {
	case <-logDone:
	case <-time.After(time.Second):
	}

	return errors.Join(runErr, stopErr, shutErr)
}

// shutdown drains with a fresh deadline: by teardown time the run ctx is
// already canceled, and a canceled ctx would abort the drain instead of
// bounding it.
func shutdown(ctx context.Context, srv *server.Server) error {
	drainCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownGrace)
	defer cancel()
	return srv.Shutdown(drainCtx)
}
