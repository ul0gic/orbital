package cmd

import (
	"context"
	"errors"
	"os"
	"testing"
)

// withArgs swaps os.Args for the duration of fn; cobra reads os.Args[1:].
func withArgs(t *testing.T, args []string, fn func()) {
	t.Helper()
	saved := os.Args
	os.Args = append([]string{"orbital"}, args...)
	t.Cleanup(func() { os.Args = saved })
	fn()
}

type hinterErr struct{}

func (hinterErr) Error() string       { return "cloudflared not found" }
func (hinterErr) InstallHint() string { return "brew install cloudflared" }

func TestExecuteExitCodes(t *testing.T) {
	dir := t.TempDir()

	cases := []struct {
		name string
		args []string
		run  RunFunc
		want int
	}{
		{
			name: "nil error exits 0",
			args: []string{dir},
			run:  func(context.Context, Config, *Session) error { return nil },
			want: 0,
		},
		{
			name: "generic error exits 1",
			args: []string{dir},
			run:  func(context.Context, Config, *Session) error { return errors.New("boom") },
			want: exitError,
		},
		{
			name: "installHinter error exits 1",
			args: []string{dir},
			run:  func(context.Context, Config, *Session) error { return hinterErr{} },
			want: exitError,
		},
		{
			name: "bad path exits 2",
			args: []string{"/no/such/path/orbital-test"},
			run:  func(context.Context, Config, *Session) error { return nil },
			want: exitUsage,
		},
		{
			name: "unknown flag exits 2",
			args: []string{"--nonsense"},
			run:  func(context.Context, Config, *Session) error { return nil },
			want: exitUsage,
		},
		{
			name: "too many args exits 2",
			args: []string{dir, dir},
			run:  func(context.Context, Config, *Session) error { return nil },
			want: exitUsage,
		},
		{
			name: "bad max-upload exits 2",
			args: []string{"--max-upload", "abc", dir},
			run:  func(context.Context, Config, *Session) error { return nil },
			want: exitUsage,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withArgs(t, tc.args, func() {
				if got := Execute(tc.run); got != tc.want {
					t.Errorf("Execute() = %d, want %d", got, tc.want)
				}
			})
		})
	}
}

func TestExecutePassesResolvedConfig(t *testing.T) {
	dir := t.TempDir()
	var seen Config
	//nolint:unparam // signature must satisfy RunFunc; this stub only records the config.
	run := func(_ context.Context, cfg Config, _ *Session) error {
		seen = cfg
		return nil
	}
	withArgs(t, []string{"--no-upload", "--max-upload", "500MB", dir}, func() {
		if code := Execute(run); code != 0 {
			t.Fatalf("Execute = %d, want 0", code)
		}
	})
	if !seen.NoUpload {
		t.Error("NoUpload not propagated")
	}
	if seen.MaxUpload != 500*1000*1000 {
		t.Errorf("MaxUpload = %d, want 500MB", seen.MaxUpload)
	}
	if seen.Path != dir {
		t.Errorf("Path = %q, want %q", seen.Path, dir)
	}
}

func TestResolveConfigPortRange(t *testing.T) {
	dir := t.TempDir()
	for _, port := range []int{-1, 70000} {
		if _, err := resolveConfig(flags{maxUpload: "2GiB", maxInbox: "16GiB", port: port}, []string{dir}); err == nil {
			t.Errorf("port %d should be rejected", port)
		}
	}
	if cfg, err := resolveConfig(flags{maxUpload: "2GiB", maxInbox: "16GiB", port: 8080}, []string{dir}); err != nil || cfg.Port != 8080 {
		t.Errorf("valid port rejected: cfg=%+v err=%v", cfg, err)
	}
}

func TestResolveConfigDefaultPath(t *testing.T) {
	cfg, err := resolveConfig(flags{maxUpload: "2GiB", maxInbox: "16GiB"}, nil)
	if err != nil {
		t.Fatalf("resolveConfig: %v", err)
	}
	if cfg.Path != "." {
		t.Errorf("default path = %q, want .", cfg.Path)
	}
}
