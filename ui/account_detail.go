package ui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lightninglabs/lightning-terminal/litrpc"
	"github.com/lnconto/lnconto/client"
)

// paymentListOverhead: title(1) + blank(2) + info box(~7) + blank(1) +
// payments header(1) + column header(1) + blank(1) + help(1) = 15
const paymentListOverhead = 15

func (m *Model) visiblePaymentRows() int {
	rows := m.safeHeight() - paymentListOverhead
	if rows < 3 {
		rows = 3
	}
	return rows
}

func (m *Model) selectedAcc() *litrpc.Account {
	if m.selectedAccount < 0 || m.selectedAccount >= len(m.accounts) {
		return nil
	}
	return m.accounts[m.selectedAccount]
}

func (m *Model) handleAccountDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.paymentsScroll > 0 {
			m.paymentsScroll--
		}
	case "down", "j":
		acc := m.selectedAcc()
		if acc != nil {
			maxScroll := len(acc.Payments) - m.visiblePaymentRows()
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.paymentsScroll < maxScroll {
				m.paymentsScroll++
			}
		}
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

	w := m.safeWidth()
	var sb strings.Builder

	// Title bar.
	sb.WriteString(styleTitleBar.Width(w).Render("⚡ lnconto — Account Detail") + "\n\n")

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
		styleMuted.Render(truncate(acc.Id, w-len(label)-10)),
		styleLabel.Render("Current Balance:"),
		formatBalanceStyled(acc.CurrentBalance),
		styleLabel.Render("Initial Balance:"),
		styleValue.Render(fmt.Sprintf("%d sats", acc.InitialBalance)),
		styleLabel.Render("Expiry:         "),
		expiryStyle.Render(expiryStr),
		styleLabel.Render("Payments:       "),
		styleValue.Render(fmt.Sprintf("%d", len(acc.Payments))),
	)
	// Width(w-2): border adds 2 outside → total = w.
	sb.WriteString(styleSection.Width(w-2).Render(infoContent) + "\n\n")

	// Payments list.
	if len(acc.Payments) > 0 {
		sb.WriteString(styleHeader.Render("Recent Payments") + "\n")
		hw, sw, aw := m.paymentColWidths()
		hdr := m.formatPaymentRow("HASH", "STATE", "AMOUNT", hw, sw, aw)
		sb.WriteString(styleLabel.Render(hdr) + "\n")

		// Most-recent-first slice.
		reversed := make([]*litrpc.AccountPayment, len(acc.Payments))
		for i, p := range acc.Payments {
			reversed[len(acc.Payments)-1-i] = p
		}

		visible := m.visiblePaymentRows()
		end := m.paymentsScroll + visible
		if end > len(reversed) {
			end = len(reversed)
		}
		for _, p := range reversed[m.paymentsScroll:end] {
			hashShort := fmt.Sprintf("%x", p.Hash)
			if len(hashShort) > hw {
				hashShort = hashShort[:hw-1] + "…"
			}
			row := m.formatPaymentRow(hashShort, p.State, fmt.Sprintf("%d sats", p.FullAmount), hw, sw, aw)
			stateStyle := styleNormal
			if strings.Contains(p.State, "FAILED") {
				stateStyle = styleRed
			} else if strings.Contains(p.State, "SUCCEEDED") || strings.Contains(p.State, "SETTLED") {
				stateStyle = styleGreen
			}
			sb.WriteString(stateStyle.Render(row) + "\n")
		}

		if len(acc.Payments) > visible {
			sb.WriteString(styleMuted.Render(fmt.Sprintf(
				"  %d–%d of %d  (↑/↓ to scroll)",
				m.paymentsScroll+1, end, len(acc.Payments),
			)) + "\n")
		}
	} else {
		sb.WriteString(styleMuted.Padding(0, 1).Render("No payments.") + "\n")
	}

	// Help bar.
	sb.WriteString("\n" + styleStatus.Width(w).Render(m.accountDetailHelp()))

	return sb.String()
}

// paymentColWidths returns hash, state, amount column widths.
// Row: hash + "  " + state + "  " + amount. Same 6-char overhead as account rows.
func (m *Model) paymentColWidths() (hashW, stateW, amountW int) {
	avail := m.safeWidth() - 6
	if avail < 30 {
		avail = 30
	}
	hashW = avail * 40 / 100
	if hashW > 22 {
		hashW = 22
	}
	stateW = avail * 35 / 100
	if stateW > 18 {
		stateW = 18
	}
	amountW = avail - hashW - stateW
	if amountW < 8 {
		amountW = 8
	}
	return
}

func (m *Model) formatPaymentRow(hash, state, amount string, hw, sw, aw int) string {
	return fmt.Sprintf("%-*s  %-*s  %-*s",
		hw, truncate(hash, hw),
		sw, truncate(state, sw),
		aw, truncate(amount, aw),
	)
}

func (m *Model) accountDetailHelp() string {
	if m.safeWidth() >= 68 {
		return "c credit   d debit   e expiry   s LNC session   m macaroon   esc back"
	}
	if m.safeWidth() >= 50 {
		return "c credit  d debit  e expiry  s session  m mac  esc"
	}
	return "c d e s m   esc back"
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

func (m *Model) doCreateSession(label string, expiryUnix uint64) tea.Cmd {
	acc := m.selectedAcc()
	if acc == nil {
		return nil
	}
	id := acc.Id
	return func() tea.Msg {
		ctx := context.Background()
		session, err := m.client.CreateAccountSession(ctx, label, id, "", expiryUnix)
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
