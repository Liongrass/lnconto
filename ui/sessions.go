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

// filteredSessions returns the sessions list with inactive (revoked/expired)
// sessions removed when hideInactiveSessions is set.
func (m *Model) filteredSessions() []*litrpc.Session {
	if !m.hideInactiveSessions {
		return m.sessions
	}
	out := make([]*litrpc.Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		if s.SessionState != litrpc.SessionState_STATE_REVOKED &&
			s.SessionState != litrpc.SessionState_STATE_EXPIRED {
			out = append(out, s)
		}
	}
	return out
}

func (m *Model) handleSessionsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	sessions := m.filteredSessions()
	switch msg.String() {
	case "up", "k":
		if m.selectedSession > 0 {
			m.selectedSession--
			m.sessionsScroll = clampScroll(m.sessionsScroll, m.selectedSession, m.visibleSessionRows())
		}
	case "down", "j":
		if m.selectedSession < len(sessions)-1 {
			m.selectedSession++
			m.sessionsScroll = clampScroll(m.sessionsScroll, m.selectedSession, m.visibleSessionRows())
		}
	case "enter", " ":
		if len(sessions) > 0 && m.selectedSession < len(sessions) {
			s := sessions[m.selectedSession]
			m.modalSessionLocalKey = s.LocalPublicKey
			m.clipboardPayload = s.PairingSecretMnemonic
			m.copied = false
			m.modal = ModalSessionDetail
			m.prevView = ViewSessions
			m.view = ViewModal
		}
	case "h":
		m.hideInactiveSessions = !m.hideInactiveSessions
		// Clamp selection to the new filtered length.
		filtered := m.filteredSessions()
		if m.selectedSession >= len(filtered) {
			if len(filtered) > 0 {
				m.selectedSession = len(filtered) - 1
			} else {
				m.selectedSession = 0
			}
		}
		m.sessionsScroll = clampScroll(m.sessionsScroll, m.selectedSession, m.visibleSessionRows())
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

	sessions := m.filteredSessions()
	header := fmt.Sprintf("Sessions (%d", len(sessions))
	if m.hideInactiveSessions && len(sessions) != len(m.sessions) {
		header += fmt.Sprintf(" of %d", len(m.sessions))
	}
	header += ")"
	if m.hideInactiveSessions {
		header += "  " + styleMuted.Render("[active only]")
	}
	sb.WriteString(styleHeader.Render(header) + "\n")

	if len(sessions) == 0 {
		sb.WriteString(styleMuted.Padding(0, 1).Render("No sessions found.") + "\n")
	} else {
		lw, tw, sw, ew := m.sessionColWidths()
		hdr := m.formatSessionRow("LABEL", "TYPE", "STATE", "EXPIRES", lw, tw, sw, ew)
		sb.WriteString(styleLabel.Render(hdr) + "\n")

		visible := m.visibleSessionRows()
		end := m.sessionsScroll + visible
		if end > len(sessions) {
			end = len(sessions)
		}
		for i := m.sessionsScroll; i < end; i++ {
			s := sessions[i]
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

		if len(sessions) > visible {
			sb.WriteString(styleMuted.Render(fmt.Sprintf(
				"  %d–%d of %d  (↑/↓ to scroll)",
				m.sessionsScroll+1, end, len(sessions),
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
	if m.safeWidth() >= 68 {
		return "↑/↓ navigate   enter select   h hide inactive   r refresh   esc back   q quit"
	}
	if m.safeWidth() >= 50 {
		return "↑/↓   enter select   h hide   r refresh   esc   q"
	}
	return "↑/↓ enter  h  r  esc  q"
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
