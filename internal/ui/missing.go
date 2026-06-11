package ui

// MissingDep describes a required external program that was not found, with the
// one-line command that installs it on the user's platform.
type MissingDep struct {
	Name       string
	InstallCmd string
	Hint       string
}

// MissingDependency renders a distinct, actionable stderr block for a missing
// external dependency: what is missing, why it matters, and the exact install
// command. Kept visually separate from generic runtime errors.
func (u *UI) MissingDependency(d MissingDep) {
	head := u.pal.fail.Render("✗ " + d.Name + " is not installed")
	u.out.write(head + "\n")
	if d.Hint != "" {
		u.out.write(u.pal.dim.Render("  "+d.Hint) + "\n")
	}
	u.out.write("\n")
	u.out.write(u.pal.label.Render("  install it with:") + "\n")
	u.out.write("    " + u.pal.url.Render(d.InstallCmd) + "\n\n")
}

// Error renders a generic error line to the diagnostic writer, distinct from
// the missing-dependency block above.
func (u *UI) Error(msg string) {
	u.out.write(u.pal.fail.Render("✗ "+msg) + "\n")
}
