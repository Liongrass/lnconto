package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lightningnetwork/lnd/lnrpc"
)

func (m *Model) handlePaymentDetailKey(_ tea.KeyMsg) (tea.Model, tea.Cmd) {
	// esc is handled centrally in handleKey; nothing else to do here.
	return m, nil
}

func (m *Model) viewPaymentDetail() string {
	if m.selectedPayment < 0 || m.selectedPayment >= len(m.enrichedPayments) {
		return "No payment selected."
	}
	pi := m.enrichedPayments[m.selectedPayment]
	p := pi.Payment

	w := m.safeWidth()
	var sb strings.Builder

	sb.WriteString(styleTitleBar.Width(w).Render("⚡ lnconto — Payment Detail") + "\n\n")

	innerW := w - 10
	if innerW < 40 {
		innerW = 40
	}

	var body strings.Builder

	// Status header.
	statusStyle, statusLabel := paymentDetailStatus(p)
	body.WriteString(statusStyle.Render("→ "+statusLabel) + "\n\n")

	// Core fields.
	body.WriteString(fieldLine("Hash", p.PaymentHash, innerW))
	body.WriteString(fieldLine("Amount", fmt.Sprintf("%d sats", p.ValueSat), innerW))
	if p.FeeSat > 0 {
		body.WriteString(fieldLine("Fee", fmt.Sprintf("%d sats", p.FeeSat), innerW))
	}
	if p.CreationTimeNs != 0 {
		t := time.Unix(p.CreationTimeNs/1_000_000_000, p.CreationTimeNs%1_000_000_000)
		body.WriteString(fieldLine("Date", t.Format("2006-01-02 15:04:05"), innerW))
	}
	if pi.Memo != "" {
		body.WriteString(fieldLine("Memo", pi.Memo, innerW))
	}
	if p.PaymentRequest != "" {
		body.WriteString(fieldLine("Invoice", wrapText(p.PaymentRequest, innerW-13), innerW))
	}
	if p.PaymentPreimage != "" {
		body.WriteString(fieldLine("Preimage", p.PaymentPreimage, innerW))
	}
	if p.FailureReason != lnrpc.PaymentFailureReason_FAILURE_REASON_NONE {
		body.WriteString(fieldLine("Failure", p.FailureReason.String(), innerW))
	}

	content := strings.TrimRight(body.String(), "\n")
	sb.WriteString(styleSection.Width(w-2).Render(content) + "\n")

	sb.WriteString("\n" + styleStatus.Width(w).Render("esc back"))
	return sb.String()
}

// fieldLine renders a "Label:     value" line, wrapping long values.
func fieldLine(label, value string, innerW int) string {
	labelStr := styleLabel.Render(fmt.Sprintf("%-10s", label+":"))
	valueW := innerW - 12
	if valueW < 20 {
		valueW = 20
	}
	lines := strings.Split(wrapText(value, valueW), "\n")
	if len(lines) == 1 {
		return labelStr + " " + styleValue.Render(value) + "\n"
	}
	var out strings.Builder
	out.WriteString(labelStr + " " + styleValue.Render(lines[0]) + "\n")
	indent := strings.Repeat(" ", 12)
	for _, l := range lines[1:] {
		out.WriteString(indent + styleValue.Render(l) + "\n")
	}
	return out.String()
}

func paymentDetailStatus(p *lnrpc.Payment) (lipgloss.Style, string) {
	switch p.Status {
	case lnrpc.Payment_SUCCEEDED:
		return styleGreen, "SUCCEEDED"
	case lnrpc.Payment_FAILED:
		return styleRed, "FAILED"
	case lnrpc.Payment_IN_FLIGHT:
		return styleWarning, "IN_FLIGHT"
	default:
		return styleMuted, p.Status.String()
	}
}

