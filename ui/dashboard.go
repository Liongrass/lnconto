package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lightninglabs/lightning-terminal/litrpc"
)

// accountListOverhead is the number of lines taken up by everything outside the
// scrollable account rows on the dashboard (title, node panel, section header,
// column header, blank line, help bar).
const accountListOverhead = 11

func (m *Model) visibleAccountRows() int {
	rows := m.safeHeight() - accountListOverhead
	if rows < 3 {
		rows = 3
	}
	return rows
}

func (m *Model) handleDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.selectedAccount > 0 {
			m.selectedAccount--
			m.accountsScroll = clampScroll(m.accountsScroll, m.selectedAccount, m.visibleAccountRows())
		}
	case "down", "j":
		if m.selectedAccount < len(m.accounts)-1 {
			m.selectedAccount++
			m.accountsScroll = clampScroll(m.accountsScroll, m.selectedAccount, m.visibleAccountRows())
		}
	case "enter", " ":
		if len(m.accounts) > 0 {
			acc := m.accounts[m.selectedAccount]
			m.prevView = ViewDashboard
			m.paymentsScroll = 0
			m.selectedPayment = 0
			m.enrichedPayments = nil
			m.paymentsLoading = true
			m.view = ViewAccountDetail
			return m, m.doLoadPayments(acc.Payments)
		}
	case "n":
		m.modal = ModalNewAccountLabel
		m.modalInput = ""
		m.modalAccountLabel = ""
		m.modalAccountBalance = 0
		m.prevView = ViewDashboard
		m.view = ViewModal
	case "x", "X":
		if len(m.accounts) > 0 {
			acc := m.accounts[m.selectedAccount]
			m.modalTitle = accountLabel(acc)
			m.modal = ModalConfirmRemove
			m.prevView = ViewDashboard
			m.view = ViewModal
		}
	case "S", "s":
		m.view = ViewLoading
		m.loadingText = "Loading sessions..."
		return m, m.fetchSessions()
	case "r":
		m.view = ViewLoading
		m.loadingText = "Refreshing..."
		return m, m.fetchAll()
	}
	return m, nil
}

func (m *Model) viewDashboard() string {
	w := m.safeWidth()
	var sb strings.Builder

	// Title bar — full width.
	sb.WriteString(styleTitleBar.Width(w).Render("⚡ lnconto — Lightning Terminal Manager") + "\n")

	// Node info panel.
	if m.nodeInfo != nil {
		sb.WriteString(m.renderNodeInfo() + "\n")
	}

	// Accounts section header.
	sb.WriteString(styleHeader.Render(fmt.Sprintf("Accounts (%d)", len(m.accounts))) + "\n")

	if len(m.accounts) == 0 {
		sb.WriteString(styleMuted.Padding(0, 1).Render("No accounts found.") + "\n")
	} else {
		// Column header row.
		lw, bw, ew := m.accountColWidths()
		hdr := m.formatAccountRow("LABEL / ID", "BALANCE", "EXPIRES", lw, bw, ew)
		sb.WriteString(styleLabel.Render(hdr) + "\n")

		// Visible slice.
		visible := m.visibleAccountRows()
		end := m.accountsScroll + visible
		if end > len(m.accounts) {
			end = len(m.accounts)
		}
		for i := m.accountsScroll; i < end; i++ {
			acc := m.accounts[i]
			row := m.formatAccountRow(accountLabel(acc), formatBalance(acc.CurrentBalance), formatExpiry(acc.ExpirationDate), lw, bw, ew)
			if i == m.selectedAccount {
				sb.WriteString(styleSelected.Width(w).Render(row) + "\n")
			} else {
				sb.WriteString(styleNormal.Render(row) + "\n")
			}
		}

		// Scroll indicator when list is longer than the viewport.
		if len(m.accounts) > visible {
			sb.WriteString(styleMuted.Render(fmt.Sprintf(
				"  %d–%d of %d  (↑/↓ to scroll)",
				m.accountsScroll+1, end, len(m.accounts),
			)) + "\n")
		}
	}

	// Help bar — pinned to bottom.
	sb.WriteString("\n" + styleStatus.Width(w).Render(m.dashboardHelp()))

	return sb.String()
}

// renderNodeInfo builds the node summary panel, stacking fields when narrow.
func (m *Model) renderNodeInfo() string {
	w := m.safeWidth()
	ni := m.nodeInfo

	syncMark := styleGreen.Render("✓ synced")
	if !ni.SyncedToChain {
		syncMark = styleWarning.Render("⚠ syncing")
	}

	pubkeyShort := ni.Pubkey
	if len(pubkeyShort) > 20 {
		pubkeyShort = pubkeyShort[:10] + "…" + pubkeyShort[len(pubkeyShort)-10:]
	}

	line1 := fmt.Sprintf("%s  %s  %s",
		styleHeader.Render(ni.Alias),
		styleMuted.Render(pubkeyShort),
		syncMark,
	)

	onchain := styleGreen.Render(fmt.Sprintf("%d sats", ni.OnchainBalance))
	offchain := styleGreen.Render(fmt.Sprintf("%d sats", ni.OffchainBalance))
	channels := styleValue.Render(fmt.Sprintf("%d", ni.NumActiveChannels))

	var line2 string
	// On narrow terminals, split balance and channel info across two lines.
	if w < 80 {
		line2 = fmt.Sprintf("%s %s   %s %s\n%s %s active",
			styleLabel.Render("on-chain:"), onchain,
			styleLabel.Render("off-chain:"), offchain,
			styleLabel.Render("channels:"), channels,
		)
	} else {
		line2 = fmt.Sprintf("%s %s   %s %s   %s %s active",
			styleLabel.Render("on-chain:"), onchain,
			styleLabel.Render("off-chain:"), offchain,
			styleLabel.Render("channels:"), channels,
		)
	}

	lines := []string{line1, line2}
	if ni.NumPendingChannels > 0 || ni.NumInactiveChannels > 0 {
		lines = append(lines, fmt.Sprintf("%s %d   %s %d   %s %d",
			styleLabel.Render("block:"), ni.BlockHeight,
			styleLabel.Render("pending:"), ni.NumPendingChannels,
			styleLabel.Render("inactive:"), ni.NumInactiveChannels,
		))
	}

	// Width(w-2): border adds 2 outside → total = w.
	return styleNodeInfo.Width(w - 2).Render(strings.Join(lines, "\n"))
}

// accountColWidths returns the three column widths for the accounts table.
// Row structure: label + "  " + balance + "  " + expiry.
// The styleNormal/styleSelected have Padding(0,1), so the row content sits
// inside a 1-char pad on each side → available content = w - 2.
// We subtract 4 more for the two "  " separators: avail = w - 6.
func (m *Model) accountColWidths() (labelW, balanceW, expiryW int) {
	avail := m.safeWidth() - 6
	if avail < 24 {
		avail = 24
	}
	// Proportional split, capped to avoid excess whitespace on wide terminals.
	labelW = avail * 45 / 100
	if labelW > 35 {
		labelW = 35
	}
	balanceW = avail * 30 / 100
	if balanceW > 18 {
		balanceW = 18
	}
	expiryW = avail - labelW - balanceW
	if expiryW > 12 {
		expiryW = 12
	}
	if expiryW < 6 {
		expiryW = 6
	}
	return
}

func (m *Model) formatAccountRow(label, balance, expiry string, lw, bw, ew int) string {
	return fmt.Sprintf("%-*s  %-*s  %-*s",
		lw, truncate(label, lw),
		bw, truncate(balance, bw),
		ew, truncate(expiry, ew),
	)
}

func (m *Model) dashboardHelp() string {
	if m.safeWidth() >= 80 {
		return "↑/↓ navigate   enter select   n new   x remove   S sessions   r refresh   q quit"
	}
	if m.safeWidth() >= 60 {
		return "↑/↓   enter   n new   x remove   S sessions   r   q"
	}
	if m.safeWidth() >= 44 {
		return "↑/↓ enter  n new  x del  S  r  q"
	}
	return "↑/↓ enter  n  x  S  r  q"
}

// ── helpers used across views ──────────────────────────────────────────────

func accountLabel(acc *litrpc.Account) string {
	if acc.Label != "" {
		return acc.Label
	}
	if len(acc.Id) > 12 {
		return acc.Id[:12] + "…"
	}
	return acc.Id
}

func formatBalance(sats int64) string {
	if sats < 0 {
		return fmt.Sprintf("-%d sats", -sats)
	}
	return fmt.Sprintf("%d sats", sats)
}

func formatExpiry(ts int64) string {
	if ts == 0 {
		return "never"
	}
	t := time.Unix(ts, 0)
	now := time.Now()
	if t.Before(now) {
		return "EXPIRED"
	}
	diff := t.Sub(now)
	if diff < 24*time.Hour {
		return fmt.Sprintf("in %dh", int(diff.Hours()))
	}
	days := int(diff.Hours() / 24)
	if days < 30 {
		return fmt.Sprintf("in %dd", days)
	}
	return t.Format("2006-01-02")
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + "…"
}

// wrapText wraps s so that no line exceeds width runes. Existing newlines in s
// are preserved; only lines that are too long get split (character-based, which
// is fine for hex; natural-language lines are usually short enough to fit).
func wrapText(s string, width int) string {
	if width <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		runes := []rune(line)
		if len(runes) <= width {
			out = append(out, line)
			continue
		}
		for len(runes) > 0 {
			n := width
			if n > len(runes) {
				n = len(runes)
			}
			out = append(out, string(runes[:n]))
			runes = runes[n:]
		}
	}
	return strings.Join(out, "\n")
}
