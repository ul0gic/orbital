package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ul0gic/sidedrop/internal/ui"
)

const (
	exitUsage = 2
	exitError = 1
)

type flags struct {
	all       bool
	noUpload  bool
	maxUpload string
	port      int
}

// Execute builds the root command, runs it with the injected core, and returns
// a process exit code. main.go passes the real run function; tests pass a fake.
func Execute(run RunFunc) int {
	out := ui.NewStderrWriter()
	u := ui.New(out)

	root := newRootCmd(run, u)
	root.SilenceErrors = true
	root.SilenceUsage = true

	err := root.Execute()
	if err == nil {
		return 0
	}
	return report(u, root, err)
}

func newRootCmd(run RunFunc, u *ui.UI) *cobra.Command {
	var f flags

	cmd := &cobra.Command{
		Use:   "sidedrop [path]",
		Short: "Ephemeral two-person file exchange over a Cloudflare quick tunnel",
		Long: "sidedrop serves a folder (or a single file) over a temporary " +
			"Cloudflare quick tunnel and prints one share link. Send the link " +
			"to one person; they download in any browser and can drop files " +
			"back. Press Ctrl-C and the link dies with the process.",
		Args:    usageArgs(cobra.MaximumNArgs(1)),
		Version: version(),
		Example: "  sidedrop\n" +
			"  sidedrop ~/share\n" +
			"  sidedrop report.pdf\n" +
			"  sidedrop --no-upload --max-upload 500MB",
		RunE: func(c *cobra.Command, args []string) error {
			cfg, err := resolveConfig(f, args)
			if err != nil {
				return usageError{err}
			}
			sess := &Session{UI: u, stdout: os.Stdout, cfg: cfg}
			return run(c.Context(), cfg, sess)
		},
	}

	cmd.Flags().BoolVar(&f.all, "all", false,
		"serve hidden and sensitive files (.ssh, .env, .git are excluded by default)")
	cmd.Flags().BoolVar(&f.noUpload, "no-upload", false,
		"disable upload-back; serve files for download only")
	cmd.Flags().StringVar(&f.maxUpload, "max-upload", "2GiB",
		"max size for a single uploaded file (e.g. 500MB, 2GB, 512MiB)")
	cmd.Flags().IntVar(&f.port, "port", 0,
		"local port to bind (0 = pick a free port automatically)")

	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return usageError{err}
	})

	cmd.SetContext(context.Background())
	return cmd
}

func resolveConfig(f flags, args []string) (Config, error) {
	path := "."
	if len(args) == 1 {
		path = args[0]
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, fmt.Errorf("path does not exist: %s", path)
		}
		return Config{}, fmt.Errorf("cannot access %s: %w", path, err)
	}

	maxUpload, err := parseSize(f.maxUpload)
	if err != nil {
		return Config{}, fmt.Errorf("--max-upload: %w", err)
	}
	if f.port < 0 || f.port > 65535 {
		return Config{}, fmt.Errorf("--port: %d out of range (0-65535)", f.port)
	}

	return Config{
		Path:      path,
		All:       f.all,
		NoUpload:  f.noUpload,
		MaxUpload: maxUpload,
		Port:      f.port,
	}, nil
}

type usageError struct{ err error }

func (e usageError) Error() string { return e.err.Error() }
func (e usageError) Unwrap() error { return e.err }

func usageArgs(validate cobra.PositionalArgs) cobra.PositionalArgs {
	return func(c *cobra.Command, args []string) error {
		if err := validate(c, args); err != nil {
			return usageError{err}
		}
		return nil
	}
}

func report(u *ui.UI, root *cobra.Command, err error) int {
	var missing installHinter
	if errors.As(err, &missing) {
		u.MissingDependency(ui.MissingDep{
			Name:       "cloudflared",
			InstallCmd: missing.InstallHint(),
			Hint:       "sidedrop opens its public tunnel through cloudflared",
		})
		return exitError
	}

	var usage usageError
	if errors.As(err, &usage) {
		u.Error(usage.Error())
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, root.UsageString())
		return exitUsage
	}

	u.Error(err.Error())
	return exitError
}
