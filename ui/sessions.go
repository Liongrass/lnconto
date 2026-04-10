package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lightninglabs/lightning-terminal/litrpc"
)

// sessionListOverhead: title(1) + blank(1) + header(1) + col header(1) +
// blank(1) + help(1) = 6
const sessionListOverhead = 6

func (m *Model) visibleSessionRows() int {
	rows := m.safeHeight() - sessionListOverhead
	if rows < 3 {
		rows = 3
	}
	return rows
}

func (m *Model) handleSessionsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.selectedSession > 0 {
			m.selectedSession--
			m.sessionsScroll = clampScroll(m.sessionsScroll, m.selectedSession, m.visibleSessionRows())
		}
	case "down", "j":
		if m.selectedSession < len(m.sessions)-1 {
			m.selectedSession++
			m.sessionsScroll = clampScroll(m.sessionsScroll, m.selectedSession, m.visibleSessionRows())
		}
	case "r":
		m.view = ViewLoading
		m.loadingText = "Refreshing sessions..."
		return m, m.fetchSessions()
	}
	return m, nil
}

func (m *Model) viewSessions() string {
	w := m.safeWidth()
	var sb strings.Builder

	sb.WriteString(styleTitleBar.Width(w).Render("⚡ lnconto — Sessions") + "\n\n")
	sb.WriteString(styleHeader.Render(fmt.Sprintf("Sessions (%d)", len(m.sessions))) + "\n")

	if len(m.sessions) == 0 {
		sb.WriteString(styleMuted.Padding(0, 1).Render("No sessions found.") + "\n")
	} else {
		lw, tw, sw, ew := m.sessionColWidths()
		hdr := m.formatSessionRow("LABEL", "TYPE", "STATE", "EXPIRES", lw, tw, sw, ew)
		sb.WriteString(styleLabel.Render(hdr) + "\n")

		visible := m.visibleSessionRows()
		end := m.sessionsScroll + visible
		if end > len(m.sessions) {
			end = len(m.sessions)
		}
		for i := m.sessionsScroll; i < end; i++ {
			s := m.sessions[i]
			row := m.formatSessionRow(
				s.Label,
				sessionTypeName(s.SessionType),
				sessionStateName(s.SessionState),
				formatExpiry(int64(s.ExpiryTimestampSeconds)),
				lw, tw, sw, ew,
			)
			if i == m.selectedSession {
				sb.WriteString(styleSelected.Width(w).Render(row) + "\n")
			} else {
				rowStyle := m.sessionRowStyle(s.SessionState)
				sb.WriteString(rowStyle.Render(row) + "\n")
			}
		}

		if len(m.sessions) > visible {
			sb.WriteString(styleMuted.Render(fmt.Sprintf(
				"  %d–%d of %d  (↑/↓ to scroll)",
				m.sessionsScroll+1, end, len(m.sessions),
			)) + "\n")
		}
	}

	sb.WriteString("\n" + styleStatus.Width(w).Render(m.sessionsHelp()))
	return sb.String()
}

// sessionColWidths returns label, type, state, expiry column widths.
// Row: label + "  " + type + "  " + state + "  " + expiry → 3 separators = 6 overhead.
func (m *Model) sessionColWidths() (labelW, typeW, stateW, expiryW int) {
	avail := m.safeWidth() - 8 // 3 × "  " separators + 2 style padding
	if avail < 36 {
		avail = 36
	}
	labelW = avail * 32 / 100
	if labelW > 28 {
		labelW = 28
	}
	typeW = avail * 22 / 100
	if typeW > 12 {
		typeW = 12
	}
	stateW = avail * 20 / 100
	if stateW > 10 {
		stateW = 10
	}
	expiryW = avail - labelW - typeW - stateW
	if expiryW > 12 {
		expiryW = 12
	}
	if expiryW < 6 {
		expiryW = 6
	}
	return
}

func (m *Model) formatSessionRow(label, typ, state, expiry string, lw, tw, sw, ew int) string {
	return fmt.Sprintf("%-*s  %-*s  %-*s  %-*s",
		lw, truncate(label, lw),
		tw, truncate(typ, tw),
		sw, truncate(state, sw),
		ew, truncate(expiry, ew),
	)
}

func (m *Model) sessionRowStyle(state litrpc.SessionState) lipgloss.Style {
	switch state {
	case litrpc.SessionState_STATE_IN_USE:
		return styleGreen
	case litrpc.SessionState_STATE_REVOKED, litrpc.SessionState_STATE_EXPIRED:
		return styleMuted
	default:
		return styleNormal
	}
}

func (m *Model) sessionsHelp() string {
	if m.safeWidth() >= 50 {
		return "↑/↓ navigate   r refresh   esc back   q quit"
	}
	return "↑/↓   r   esc   q"
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
