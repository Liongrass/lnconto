package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lightninglabs/lightning-terminal/litrpc"
)

func (m *Model) handleDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.selectedAccount > 0 {
			m.selectedAccount--
		}
	case "down", "j":
		if m.selectedAccount < len(m.accounts)-1 {
			m.selectedAccount++
		}
	case "enter", " ":
		if len(m.accounts) > 0 {
			m.prevView = ViewDashboard
			m.view = ViewAccountDetail
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
	var sb strings.Builder

	// Title bar.
	title := styleTitleBar.Width(m.width).Render("⚡ lnconto — Lightning Terminal Manager")
	sb.WriteString(title + "\n")

	// Node info panel.
	if m.nodeInfo != nil {
		ni := m.nodeInfo
		syncMark := styleGreen.Render("✓ synced")
		if !ni.SyncedToChain {
			syncMark = styleWarning.Render("⚠ syncing")
		}

		pubkeyShort := ni.Pubkey
		if len(pubkeyShort) > 20 {
			pubkeyShort = pubkeyShort[:10] + "…" + pubkeyShort[len(pubkeyShort)-10:]
		}

		infoLines := []string{
			fmt.Sprintf("%s  %s  %s",
				styleHeader.Render(ni.Alias),
				styleMuted.Render(pubkeyShort),
				syncMark,
			),
			fmt.Sprintf("%s %s   %s %s   %s %s active channels",
				styleLabel.Render("on-chain:"),
				styleGreen.Render(fmt.Sprintf("%d sats", ni.OnchainBalance)),
				styleLabel.Render("off-chain:"),
				styleGreen.Render(fmt.Sprintf("%d sats", ni.OffchainBalance)),
				styleLabel.Render("channels:"),
				styleValue.Render(fmt.Sprintf("%d", ni.NumActiveChannels)),
			),
		}
		if ni.NumPendingChannels > 0 || ni.NumInactiveChannels > 0 {
			infoLines = append(infoLines,
				fmt.Sprintf("%s %d   %s %d   %s %d",
					styleLabel.Render("block:"), ni.BlockHeight,
					styleLabel.Render("pending:"), ni.NumPendingChannels,
					styleLabel.Render("inactive:"), ni.NumInactiveChannels,
				),
			)
		}
		panel := styleNodeInfo.Width(m.width - 4).Render(strings.Join(infoLines, "\n"))
		sb.WriteString(panel + "\n")
	}

	// Accounts list.
	sb.WriteString(styleHeader.Render(fmt.Sprintf("Accounts (%d)", len(m.accounts))) + "\n")

	if len(m.accounts) == 0 {
		sb.WriteString(styleMuted.Padding(0, 2).Render("No accounts found.") + "\n")
	} else {
		// Table header.
		hdr := renderAccountRow("LABEL / ID", "BALANCE", "EXPIRES", false)
		sb.WriteString(styleLabel.Render(hdr) + "\n")

		for i, acc := range m.accounts {
			selected := i == m.selectedAccount
			sb.WriteString(renderAccountRow(accountLabel(acc), formatBalance(acc.CurrentBalance), formatExpiry(acc.ExpirationDate), selected) + "\n")
		}
	}

	// Help bar.
	help := styleHelp.Render("↑/↓ navigate   enter select   S sessions   r refresh   q quit")
	sb.WriteString("\n" + styleStatus.Width(m.width).Render(help))

	return sb.String()
}

func renderAccountRow(label, balance, expiry string, selected bool) string {
	labelCol := fmt.Sprintf("%-30s", truncate(label, 30))
	balanceCol := fmt.Sprintf("%-20s", truncate(balance, 20))
	expiryCol := truncate(expiry, 20)
	row := fmt.Sprintf("  %s  %s  %s", labelCol, balanceCol, expiryCol)
	if selected {
		return styleSelected.Render(row)
	}
	return styleNormal.Render(row)
}

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
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func center(s string, width int) string {
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(s)
}
