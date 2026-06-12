package cmd

// Config is the resolved CLI surface the core run function consumes. cmd owns
// parsing and validation; everything downstream reads these settled values.
type Config struct {
	Path      string
	All       bool
	NoUpload  bool
	MaxUpload int64
	MaxInbox  int64
	Port      int
}

// installHinter is satisfied by the tunnel layer's cloudflared-not-found error
// (*tunnel.ErrCloudflaredNotFound). cmd detects it via errors.As to render the
// install-hint block instead of a generic error. Matched structurally so cmd
// does not import the tunnel package.
type installHinter interface {
	error
	InstallHint() string
}
