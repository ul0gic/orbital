package ui

import "github.com/charmbracelet/lipgloss"

type palette struct {
	enabled  bool
	title    lipgloss.Style
	label    lipgloss.Style
	url      lipgloss.Style
	dim      lipgloss.Style
	frame    lipgloss.Style
	ts       lipgloss.Style
	download lipgloss.Style
	upload   lipgloss.Style
	ok       lipgloss.Style
	visitor  lipgloss.Style
	ready    lipgloss.Style
	fail     lipgloss.Style
}

// newPalette builds the style set. When color is disabled (piped output or
// NO_COLOR) every style is the identity so rendered strings carry no escapes.
func newPalette(color bool) palette {
	if !color {
		plain := lipgloss.NewStyle()
		return palette{
			title: plain, label: plain, url: plain, dim: plain, frame: plain,
			ts: plain, download: plain, upload: plain, ok: plain,
			visitor: plain, ready: plain, fail: plain,
		}
	}
	return palette{
		enabled:  true,
		title:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213")),
		label:    lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		url:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("159")),
		dim:      lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		frame:    lipgloss.NewStyle().Foreground(lipgloss.Color("99")),
		ts:       lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		download: lipgloss.NewStyle().Foreground(lipgloss.Color("39")),
		upload:   lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		ok:       lipgloss.NewStyle().Foreground(lipgloss.Color("78")),
		visitor:  lipgloss.NewStyle().Foreground(lipgloss.Color("141")),
		ready:    lipgloss.NewStyle().Foreground(lipgloss.Color("78")),
		fail:     lipgloss.NewStyle().Foreground(lipgloss.Color("203")),
	}
}
