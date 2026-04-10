package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	colorPrimary   = lipgloss.Color("#F7931A") // Bitcoin orange
	colorSecondary = lipgloss.Color("#7D7D7D")
	colorSuccess   = lipgloss.Color("#3EBA79")
	colorWarning   = lipgloss.Color("#F7CB45")
	colorDanger    = lipgloss.Color("#E74C3C")
	colorMuted     = lipgloss.Color("#4A4A4A")
	colorBg        = lipgloss.Color("#1A1A1A")
	colorBgAlt     = lipgloss.Color("#222222")
	colorBorder    = lipgloss.Color("#333333")
	colorSelected  = lipgloss.Color("#F7931A")
	colorText      = lipgloss.Color("#E0E0E0")

	styleTitleBar = lipgloss.NewStyle().
			Background(colorPrimary).
			Foreground(lipgloss.Color("#000000")).
			Bold(true).
			Padding(0, 2)

	styleNodeInfo = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 2).
			Margin(0, 0, 1, 0)

	styleSection = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	styleSelected = lipgloss.NewStyle().
			Background(colorSelected).
			Foreground(lipgloss.Color("#000000")).
			Bold(true).
			Padding(0, 1)

	styleNormal = lipgloss.NewStyle().
			Foreground(colorText).
			Padding(0, 1)

	styleLabel = lipgloss.NewStyle().
			Foreground(colorSecondary)

	styleValue = lipgloss.NewStyle().
			Foreground(colorText).
			Bold(true)

	styleGreen = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Bold(true)

	styleRed = lipgloss.NewStyle().
			Foreground(colorDanger).
			Bold(true)

	styleWarning = lipgloss.NewStyle().
			Foreground(colorWarning)

	styleMuted = lipgloss.NewStyle().
			Foreground(colorMuted)

	styleHelp = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Padding(0, 1)

	styleInput = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(0, 1).
			Width(40)

	styleModal = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(1, 3).
			Background(colorBgAlt)

	styleHeader = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	stylePubkey = lipgloss.NewStyle().
			Foreground(colorSecondary).
			MaxWidth(16)

	styleStatus = lipgloss.NewStyle().
			Background(colorBgAlt).
			Foreground(colorSecondary).
			Padding(0, 2)
)

func formatSats(sats int64) string {
	if sats < 0 {
		return styleRed.Render(fmt.Sprintf("-%d sats", -sats))
	}
	s := fmt.Sprintf("%d sats", sats)
	if sats > 1_000_000 {
		return styleGreen.Render(s)
	}
	return styleValue.Render(s)
}
