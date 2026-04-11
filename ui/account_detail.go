package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lightninglabs/lightning-terminal/litrpc"
	"github.com/lnconto/lnconto/client"
)

// paymentListOverhead: title(1) + blank(1) + info box(~6) + blank(1) +
// payments header(1) + column header(1) + blank(1) + help(1) = 13
const paymentListOverhead = 13

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
		if m.selectedPayment > 0 {
			m.selectedPayment--
			m.paymentsScroll = clampScroll(m.paymentsScroll, m.selectedPayment, m.visiblePaymentRows())
		}
	case "down", "j":
		if m.selectedPayment < len(m.enrichedPayments)-1 {
			m.selectedPayment++
			m.paymentsScroll = clampScroll(m.paymentsScroll, m.selectedPayment, m.visiblePaymentRows())
		}
	case "enter", " ":
		if len(m.enrichedPayments) > 0 && m.selectedPayment < len(m.enrichedPayments) {
			m.view = ViewPaymentDetail
		}
	case "l":
		acc := m.selectedAcc()
		if acc != nil && !m.paymentsLoading {
			m.paymentsLoading = true
			return m, m.doLoadMorePayments(acc.Payments)
		}
	case "r":
		if !m.paymentsLoading {
			return m, m.doGetAccount()
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

	// Payments section.
	if m.paymentsLoading {
		sb.WriteString(styleMuted.Padding(0, 1).Render("Loading payments...") + "\n")
	} else if len(m.enrichedPayments) == 0 {
		if len(acc.Payments) > 0 {
			sb.WriteString(styleMuted.Padding(0, 1).Render("No payment details available.") + "\n")
		} else {
			sb.WriteString(styleMuted.Padding(0, 1).Render("No payments.") + "\n")
		}
	} else {
		sb.WriteString(styleHeader.Render("Payments") + "\n")
		dw, tw, aw, mw := m.paymentColWidths()
		hdr := m.formatPaymentRow("DIR  DATE", "STATUS", "AMOUNT", "MEMO", dw, tw, aw, mw)
		sb.WriteString(styleLabel.Render(hdr) + "\n")

		visible := m.visiblePaymentRows()
		end := m.paymentsScroll + visible
		if end > len(m.enrichedPayments) {
			end = len(m.enrichedPayments)
		}

		for i := m.paymentsScroll; i < end; i++ {
			pi := m.enrichedPayments[i]
			dirStr := paymentDirStr(pi.Direction)
			dateStr := formatPaymentTime(pi.TimestampNs)
			dirDate := dirStr + " " + dateStr
			row := m.formatPaymentRow(dirDate, pi.Status, formatBalance(pi.AmountSat), pi.Memo, dw, tw, aw, mw)
			if i == m.selectedPayment {
				sb.WriteString(styleSelected.Width(w).Render(row) + "\n")
			} else {
				sb.WriteString(paymentRowStyle(pi).Render(row) + "\n")
			}
		}

		if len(m.enrichedPayments) > visible {
			sb.WriteString(styleMuted.Render(fmt.Sprintf(
				"  %d–%d of %d  (↑/↓ to scroll)",
				m.paymentsScroll+1, end, len(m.enrichedPayments),
			)) + "\n")
		}
	}

	// Help bar.
	sb.WriteString("\n" + styleStatus.Width(w).Render(m.accountDetailHelp()))

	return sb.String()
}

// paymentColWidths returns direction+date, status, amount, memo column widths.
func (m *Model) paymentColWidths() (dirW, statusW, amountW, memoW int) {
	avail := m.safeWidth() - 8 // 3×"  " separators + 2 style padding
	if avail < 40 {
		avail = 40
	}
	dirW = 14 // "← 2024-01-01" or "→ 2024-01-01"
	statusW = avail * 20 / 100
	if statusW > 12 {
		statusW = 12
	}
	amountW = avail * 20 / 100
	if amountW > 14 {
		amountW = 14
	}
	memoW = avail - dirW - statusW - amountW
	if memoW < 6 {
		memoW = 6
	}
	return
}

func (m *Model) formatPaymentRow(dir, status, amount, memo string, dw, sw, aw, mw int) string {
	return fmt.Sprintf("%-*s  %-*s  %-*s  %-*s",
		dw, truncate(dir, dw),
		sw, truncate(status, sw),
		aw, truncate(amount, aw),
		mw, truncate(memo, mw),
	)
}

func (m *Model) accountDetailHelp() string {
	if m.safeWidth() >= 80 {
		return "↑/↓ navigate   enter detail   l load more   r refresh   c credit   d debit   e expiry   s session   m mac   esc back"
	}
	if m.safeWidth() >= 68 {
		return "↑/↓ enter   l more   r ref   c credit   d debit   e expiry   s session   m mac   esc"
	}
	if m.safeWidth() >= 50 {
		return "↑/↓ enter  l  r  c  d  e  s  m  esc"
	}
	return "↑/↓ l r c d e s m  esc"
}

func formatBalanceStyled(sats int64) string {
	if sats < 0 {
		return styleRed.Render(fmt.Sprintf("-%d sats", -sats))
	}
	return styleGreen.Render(fmt.Sprintf("%d sats", sats))
}

func paymentDirStr(d client.PaymentDirection) string {
	switch d {
	case client.DirectionIncoming:
		return "←"
	case client.DirectionOutgoing:
		return "→"
	default:
		return "?"
	}
}

func paymentRowStyle(pi *client.PaymentInfo) lipgloss.Style {
	switch pi.Direction {
	case client.DirectionIncoming:
		return styleGreen
	case client.DirectionOutgoing:
		if strings.Contains(pi.Status, "FAILED") {
			return styleRed
		}
		return styleNormal
	default:
		return styleMuted
	}
}

func formatPaymentTime(nsec int64) string {
	if nsec == 0 {
		return "unknown"
	}
	t := time.Unix(nsec/1_000_000_000, nsec%1_000_000_000)
	now := time.Now()
	diff := now.Sub(t)
	if diff < 24*time.Hour {
		return t.Format("15:04")
	}
	return t.Format("2006-01-02")
}

func (m *Model) doGetAccount() tea.Cmd {
	acc := m.selectedAcc()
	if acc == nil {
		return nil
	}
	id := acc.Id
	return func() tea.Msg {
		ctx := context.Background()
		updated, err := m.client.GetAccount(ctx, id)
		if err != nil {
			return msgError{err}
		}
		return msgAccountUpdated{updated}
	}
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
