package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lnconto/lnconto/client"
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

	w := m.safeWidth()
	var sb strings.Builder

	sb.WriteString(styleTitleBar.Width(w).Render("⚡ lnconto — Payment Detail") + "\n\n")

	innerW := w - 10
	if innerW < 40 {
		innerW = 40
	}

	var body strings.Builder

	if pi.IsIncoming() {
		body.WriteString(m.viewInvoiceDetail(pi, innerW))
	} else {
		body.WriteString(m.viewOutgoingDetail(pi, innerW))
	}

	content := strings.TrimRight(body.String(), "\n")
	sb.WriteString(styleSection.Width(w-2).Render(content) + "\n")

	sb.WriteString("\n" + styleStatus.Width(w).Render("esc back"))
	return sb.String()
}

func (m *Model) viewOutgoingDetail(pi *client.PaymentInfo, innerW int) string {
	p := pi.Payment
	var body strings.Builder

	statusStyle, statusLabel := paymentDetailStatus(p)
	body.WriteString(statusStyle.Render("→ Outgoing  "+statusLabel) + "\n\n")

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
	return body.String()
}

func (m *Model) viewInvoiceDetail(pi *client.PaymentInfo, innerW int) string {
	inv := pi.Invoice
	var body strings.Builder

	statusStyle, statusLabel := invoiceDetailStatus(inv)
	body.WriteString(statusStyle.Render("← Incoming  "+statusLabel) + "\n\n")

	body.WriteString(fieldLine("Hash", fmt.Sprintf("%x", inv.RHash), innerW))
	amt := inv.AmtPaidSat
	if amt == 0 {
		amt = inv.Value
	}
	body.WriteString(fieldLine("Amount", fmt.Sprintf("%d sats", amt), innerW))
	if inv.CreationDate != 0 {
		t := time.Unix(inv.CreationDate, 0)
		body.WriteString(fieldLine("Created", t.Format("2006-01-02 15:04:05"), innerW))
	}
	if inv.SettleDate != 0 {
		t := time.Unix(inv.SettleDate, 0)
		body.WriteString(fieldLine("Settled", t.Format("2006-01-02 15:04:05"), innerW))
	}
	if pi.Memo != "" {
		body.WriteString(fieldLine("Memo", pi.Memo, innerW))
	}
	if inv.PaymentRequest != "" {
		body.WriteString(fieldLine("Invoice", wrapText(inv.PaymentRequest, innerW-13), innerW))
	}
	return body.String()
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

func invoiceDetailStatus(inv *lnrpc.Invoice) (lipgloss.Style, string) {
	switch inv.State {
	case lnrpc.Invoice_SETTLED:
		return styleGreen, "SETTLED"
	case lnrpc.Invoice_CANCELED:
		return styleRed, "CANCELED"
	case lnrpc.Invoice_ACCEPTED:
		return styleWarning, "ACCEPTED"
	default:
		return styleNormal, "OPEN"
	}
}


