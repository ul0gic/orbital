package tunnel

type ErrCloudflaredNotFound struct {
	GOOS string
}

func (e *ErrCloudflaredNotFound) Error() string {
	return "cloudflared not found on PATH"
}

// InstallHint returns a one-line install command for the target platform.
func (e *ErrCloudflaredNotFound) InstallHint() string {
	switch e.GOOS {
	case "darwin":
		return "brew install cloudflared"
	case "linux":
		return "see https://pkg.cloudflare.com/ or: curl -fsSL https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 -o /usr/local/bin/cloudflared && chmod +x /usr/local/bin/cloudflared"
	default:
		return "install cloudflared: https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/"
	}
}
