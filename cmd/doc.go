package cmd

// Version is overridable at build time via -ldflags "-X .../cmd.Version=...".
var Version = "dev"

func version() string { return Version }
