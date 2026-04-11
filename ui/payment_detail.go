package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lnconto/lnconto/client"
)

func (m *Model) handlePaymentDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Any key other than esc goes back (esc is handled in handleKey).
	return m, nil
}

func (m *Model) viewPaymentDetail() string {
	if m.selectedPayment < 0 || m.selectedPayment >= len(m.enrichedPayments) {
		return "No payment selected."
	}
	pi := m.enrichedPayments[m.selectedPayment]

	w := m.safeWidth()
	var sb strings.Builder

	sb.WriteString(styleTitleBar.Width(w).Render("⚡ lnconto — Payment Detail") + "\n\n")

	innerW := w - 10
	if innerW < 40 {
		innerW = 40
	}

	var body strings.Builder

	// Direction and status header.
	dirLabel, dirColor := paymentDetailDir(pi)
	body.WriteString(dirColor.Render(dirLabel) + "  " + styleMuted.Render(pi.Status) + "\n\n")

	// Common fields.
	body.WriteString(fieldLine("Hash", pi.HashHex, innerW))
	body.WriteString(fieldLine("Amount", fmt.Sprintf("%d sats", pi.AmountSat), innerW))
	if pi.FeeSat > 0 {
		body.WriteString(fieldLine("Fee", fmt.Sprintf("%d sats", pi.FeeSat), innerW))
	}
	if pi.TimestampNs != 0 {
		t := time.Unix(pi.TimestampNs/1_000_000_000, pi.TimestampNs%1_000_000_000)
		body.WriteString(fieldLine("Time", t.Format("2006-01-02 15:04:05"), innerW))
	}
	if pi.Memo != "" {
		body.WriteString(fieldLine("Memo", pi.Memo, innerW))
	}

	// Invoice-specific fields.
	if pi.Invoice != nil {
		inv := pi.Invoice
		if inv.SettleDate > 0 {
			t := time.Unix(inv.SettleDate, 0)
			body.WriteString(fieldLine("Settled", t.Format("2006-01-02 15:04:05"), innerW))
		}
		if inv.PaymentRequest != "" {
			body.WriteString(fieldLine("Invoice", wrapText(inv.PaymentRequest, innerW-18), innerW))
		}
		if inv.IsKeysend {
			body.WriteString(fieldLine("Type", "Keysend", innerW))
		}
	}

	// Payment-specific fields.
	if pi.Payment != nil {
		p := pi.Payment
		if p.PaymentRequest != "" {
			body.WriteString(fieldLine("Invoice", wrapText(p.PaymentRequest, innerW-18), innerW))
		}
		if p.PaymentPreimage != "" {
			body.WriteString(fieldLine("Preimage", p.PaymentPreimage, innerW))
		}
		if p.FailureReason.String() != "FAILURE_REASON_NONE" {
			body.WriteString(fieldLine("Failure", p.FailureReason.String(), innerW))
		}
	}

	content := strings.TrimRight(body.String(), "\n")
	sb.WriteString(styleSection.Width(w-2).Render(content) + "\n")

	sb.WriteString("\n" + styleStatus.Width(w).Render("esc back"))
	return sb.String()
}

// fieldLine renders a labeled field, wrapping the value if needed.
func fieldLine(label, value string, innerW int) string {
	labelStr := styleLabel.Render(fmt.Sprintf("%-10s", label+":"))
	valueW := innerW - 12 // label(10) + ": "(2)
	if valueW < 20 {
		valueW = 20
	}
	lines := strings.Split(wrapText(value, valueW), "\n")
	if len(lines) == 1 {
		return labelStr + " " + styleValue.Render(value) + "\n"
	}
	// Multi-line: indent continuation lines.
	var sb strings.Builder
	sb.WriteString(labelStr + " " + styleValue.Render(lines[0]) + "\n")
	indent := strings.Repeat(" ", 12)
	for _, l := range lines[1:] {
		sb.WriteString(indent + styleValue.Render(l) + "\n")
	}
	return sb.String()
}

func paymentDetailDir(pi *client.PaymentInfo) (string, lipgloss.Style) {
	switch pi.Direction {
	case client.DirectionIncoming:
		return "← Incoming", styleGreen
	case client.DirectionOutgoing:
		return "→ Outgoing", styleWarning
	default:
		return "? Unknown", styleMuted
	}
}
