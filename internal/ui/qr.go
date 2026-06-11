package ui

import "github.com/mdp/qrterminal/v3"

// QR writes a compact QR code of the share URL to the diagnostic writer.
// It is suppressed entirely when the destination is not a TTY so piped output
// stays clean.
func (u *UI) QR(url string) {
	if !u.out.IsTTY() {
		return
	}
	qrterminal.GenerateWithConfig(url, qrterminal.Config{
		Level:      qrterminal.L,
		Writer:     u.out.w,
		HalfBlocks: true,
		QuietZone:  1,
	})
	u.out.write("\n")
}
