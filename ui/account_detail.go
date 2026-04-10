package ui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lightninglabs/lightning-terminal/litrpc"
	"github.com/lnconto/lnconto/client"
)

func (m *Model) selectedAcc() *litrpc.Account {
	if m.selectedAccount < 0 || m.selectedAccount >= len(m.accounts) {
		return nil
	}
	return m.accounts[m.selectedAccount]
}

func (m *Model) handleAccountDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "c":
		m.modal = ModalCredit
		m.modalTitle = "Credit Account"
		m.modalInput = ""
		m.prevView = ViewAccountDetail
		m.view = ViewModal
	case "d":
		m.modal = ModalDebit
		m.modalTitle = "Debit Account"
		m.modalInput = ""
		m.prevView = ViewAccountDetail
		m.view = ViewModal
	case "e":
		m.modal = ModalExpiry
		m.modalTitle = "Set Expiry (YYYY-MM-DD, or 0 for never)"
		m.modalInput = ""
		m.prevView = ViewAccountDetail
		m.view = ViewModal
	case "s":
		m.modal = ModalNewSession
		m.modalTitle = "Session Label"
		m.modalInput = ""
		m.prevView = ViewAccountDetail
		m.view = ViewModal
	case "m":
		m.modal = ModalMacaroonType
		m.modalInput = ""
		m.prevView = ViewAccountDetail
		m.view = ViewModal
	}
	return m, nil
}

func (m *Model) viewAccountDetail() string {
	acc := m.selectedAcc()
	if acc == nil {
		return "No account selected."
	}

	var sb strings.Builder

	// Header.
	title := styleTitleBar.Width(m.width).Render("⚡ lnconto — Account Detail")
	sb.WriteString(title + "\n\n")

	// Account info box.
	label := acc.Label
	if label == "" {
		label = "(no label)"
	}
	expiryStr := formatExpiry(acc.ExpirationDate)
	expiryStyle := styleValue
	if acc.ExpirationDate != 0 {
		if expiryStr == "EXPIRED" {
			expiryStyle = styleRed
		} else {
			expiryStyle = styleWarning
		}
	}

	infoContent := fmt.Sprintf(
		"%s  %s\n\n%s %s\n%s %s\n%s %s\n%s %s",
		styleHeader.Render(label),
		styleMuted.Render(acc.Id),
		styleLabel.Render("Current Balance:"),
		formatBalanceStyled(acc.CurrentBalance),
		styleLabel.Render("Initial Balance:"),
		styleValue.Render(fmt.Sprintf("%d sats", acc.InitialBalance)),
		styleLabel.Render("Expiry:         "),
		expiryStyle.Render(expiryStr),
		styleLabel.Render("Payments:       "),
		styleValue.Render(fmt.Sprintf("%d", len(acc.Payments))),
	)
	sb.WriteString(styleSection.Width(m.width-4).Render(infoContent) + "\n\n")

	// Payments list.
	if len(acc.Payments) > 0 {
		sb.WriteString(styleHeader.Render("Recent Payments") + "\n")
		hdr := fmt.Sprintf("  %-20s  %-15s  %s", "HASH", "STATE", "AMOUNT")
		sb.WriteString(styleLabel.Render(hdr) + "\n")

		max := len(acc.Payments)
		if max > 15 {
			max = 15
		}
		// Show most recent first (last in slice).
		for i := len(acc.Payments) - 1; i >= len(acc.Payments)-max; i-- {
			p := acc.Payments[i]
			hashShort := fmt.Sprintf("%x", p.Hash)
			if len(hashShort) > 20 {
				hashShort = hashShort[:10] + "…"
			}
			stateStyle := styleValue
			if strings.Contains(p.State, "FAILED") {
				stateStyle = styleRed
			} else if strings.Contains(p.State, "SUCCEEDED") || strings.Contains(p.State, "SETTLED") {
				stateStyle = styleGreen
			}
			row := fmt.Sprintf("  %-20s  %-15s  %d sats",
				hashShort,
				truncate(p.State, 15),
				p.FullAmount,
			)
			sb.WriteString(stateStyle.Render(row) + "\n")
		}
	} else {
		sb.WriteString(styleMuted.Padding(0, 2).Render("No payments.") + "\n")
	}

	// Help bar.
	helpItems := []string{
		"c credit",
		"d debit",
		"e expiry",
		"s new LNC session",
		"m macaroon",
		"esc back",
	}
	help := styleHelp.Render(strings.Join(helpItems, "   "))
	sb.WriteString("\n" + styleStatus.Width(m.width).Render(help))

	return sb.String()
}

func formatBalanceStyled(sats int64) string {
	if sats < 0 {
		return styleRed.Render(fmt.Sprintf("-%d sats", -sats))
	}
	return styleGreen.Render(fmt.Sprintf("%d sats", sats))
}

func (m *Model) doCreditAccount(amount uint64) tea.Cmd {
	acc := m.selectedAcc()
	if acc == nil {
		return nil
	}
	id := acc.Id
	return func() tea.Msg {
		ctx := context.Background()
		updated, err := m.client.CreditAccount(ctx, id, amount)
		if err != nil {
			return msgError{err}
		}
		return msgAccountUpdated{updated}
	}
}

func (m *Model) doDebitAccount(amount uint64) tea.Cmd {
	acc := m.selectedAcc()
	if acc == nil {
		return nil
	}
	id := acc.Id
	return func() tea.Msg {
		ctx := context.Background()
		updated, err := m.client.DebitAccount(ctx, id, amount)
		if err != nil {
			return msgError{err}
		}
		return msgAccountUpdated{updated}
	}
}

func (m *Model) doUpdateExpiry(expiryUnix int64) tea.Cmd {
	acc := m.selectedAcc()
	if acc == nil {
		return nil
	}
	id := acc.Id
	return func() tea.Msg {
		ctx := context.Background()
		updated, err := m.client.UpdateExpiry(ctx, id, expiryUnix)
		if err != nil {
			return msgError{err}
		}
		return msgAccountUpdated{updated}
	}
}

func (m *Model) doCreateSession(label string) tea.Cmd {
	acc := m.selectedAcc()
	if acc == nil {
		return nil
	}
	id := acc.Id
	return func() tea.Msg {
		ctx := context.Background()
		session, err := m.client.CreateAccountSession(ctx, label, id, "", 0)
		if err != nil {
			return msgError{err}
		}
		return msgSessionCreated{session}
	}
}

func (m *Model) doBakeMacaroon(t client.MacaroonType) tea.Cmd {
	acc := m.selectedAcc()
	if acc == nil {
		return nil
	}
	id := acc.Id
	return func() tea.Msg {
		ctx := context.Background()
		hex, err := m.client.BakeAccountMacaroon(ctx, id, t)
		if err != nil {
			return msgError{err}
		}
		return msgMacaroon{hex}
	}
}
