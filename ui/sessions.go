package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lightninglabs/lightning-terminal/litrpc"
)

func (m *Model) handleSessionsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.selectedSession > 0 {
			m.selectedSession--
		}
	case "down", "j":
		if m.selectedSession < len(m.sessions)-1 {
			m.selectedSession++
		}
	case "r":
		m.view = ViewLoading
		m.loadingText = "Refreshing sessions..."
		return m, m.fetchSessions()
	}
	return m, nil
}

func (m *Model) viewSessions() string {
	var sb strings.Builder

	title := styleTitleBar.Width(m.width).Render("⚡ lnconto — Sessions")
	sb.WriteString(title + "\n\n")

	sb.WriteString(styleHeader.Render(fmt.Sprintf("Sessions (%d)", len(m.sessions))) + "\n")

	if len(m.sessions) == 0 {
		sb.WriteString(styleMuted.Padding(0, 2).Render("No sessions found.") + "\n")
	} else {
		hdr := fmt.Sprintf("  %-25s  %-22s  %-12s  %s", "LABEL", "TYPE", "STATE", "EXPIRES")
		sb.WriteString(styleLabel.Render(hdr) + "\n")

		for i, s := range m.sessions {
			selected := i == m.selectedSession
			row := renderSessionRow(s, selected)
			sb.WriteString(row + "\n")
		}
	}

	help := styleHelp.Render("↑/↓ navigate   r refresh   esc back   q quit")
	sb.WriteString("\n" + styleStatus.Width(m.width).Render(help))

	return sb.String()
}

func renderSessionRow(s *litrpc.Session, selected bool) string {
	label := truncate(s.Label, 25)
	typeName := sessionTypeName(s.SessionType)
	stateName := sessionStateName(s.SessionState)
	expiry := formatExpiry(int64(s.ExpiryTimestampSeconds))

	row := fmt.Sprintf("  %-25s  %-22s  %-12s  %s", label, typeName, stateName, expiry)

	var stateStyle = styleNormal
	switch s.SessionState {
	case litrpc.SessionState_STATE_IN_USE:
		stateStyle = styleGreen
	case litrpc.SessionState_STATE_REVOKED, litrpc.SessionState_STATE_EXPIRED:
		stateStyle = styleMuted
	}

	if selected {
		return styleSelected.Render(row)
	}
	return stateStyle.Render(row)
}

func sessionTypeName(t litrpc.SessionType) string {
	switch t {
	case litrpc.SessionType_TYPE_MACAROON_READONLY:
		return "Readonly"
	case litrpc.SessionType_TYPE_MACAROON_ADMIN:
		return "Admin"
	case litrpc.SessionType_TYPE_MACAROON_CUSTOM:
		return "Custom"
	case litrpc.SessionType_TYPE_MACAROON_ACCOUNT:
		return "Account"
	case litrpc.SessionType_TYPE_AUTOPILOT:
		return "Autopilot"
	default:
		return fmt.Sprintf("Type(%d)", int(t))
	}
}

func sessionStateName(s litrpc.SessionState) string {
	switch s {
	case litrpc.SessionState_STATE_CREATED:
		return "Created"
	case litrpc.SessionState_STATE_IN_USE:
		return "In Use"
	case litrpc.SessionState_STATE_REVOKED:
		return "Revoked"
	case litrpc.SessionState_STATE_EXPIRED:
		return "Expired"
	default:
		return fmt.Sprintf("State(%d)", int(s))
	}
}
