package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lnconto/lnconto/client"
)

func (m *Model) handleModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.modal {
	case ModalCredit, ModalDebit, ModalExpiry, ModalNewSession:
		return m.handleTextInputKey(msg)
	case ModalMacaroonType:
		return m.handleMacaroonTypeKey(msg)
	case ModalMacaroonResult:
		// any key closes the result
		m.modal = ModalNone
		m.modalResult = ""
		m.view = m.prevView
		return m, nil
	}
	return m, nil
}

func (m *Model) handleTextInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		return m.submitModal()
	case "backspace", "ctrl+h":
		if len(m.modalInput) > 0 {
			m.modalInput = m.modalInput[:len(m.modalInput)-1]
		}
	case "ctrl+u":
		m.modalInput = ""
	default:
		if len(msg.Runes) == 1 {
			m.modalInput += string(msg.Runes)
		}
	}
	return m, nil
}

func (m *Model) handleMacaroonTypeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "1":
		m.view = ViewLoading
		m.loadingText = "Baking macaroon..."
		return m, m.doBakeMacaroon(client.MacaroonTypeAccount)
	case "2":
		m.view = ViewLoading
		m.loadingText = "Baking macaroon..."
		return m, m.doBakeMacaroon(client.MacaroonTypeAccountReadonly)
	case "3":
		m.view = ViewLoading
		m.loadingText = "Baking macaroon..."
		return m, m.doBakeMacaroon(client.MacaroonTypeInvoice)
	}
	return m, nil
}

func (m *Model) submitModal() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.modalInput)

	switch m.modal {
	case ModalCredit:
		amount, err := strconv.ParseUint(input, 10, 64)
		if err != nil || amount == 0 {
			m.modalInput = ""
			return m, nil
		}
		m.view = ViewLoading
		m.loadingText = "Crediting account..."
		return m, m.doCreditAccount(amount)

	case ModalDebit:
		amount, err := strconv.ParseUint(input, 10, 64)
		if err != nil || amount == 0 {
			m.modalInput = ""
			return m, nil
		}
		m.view = ViewLoading
		m.loadingText = "Debiting account..."
		return m, m.doDebitAccount(amount)

	case ModalExpiry:
		if input == "0" || input == "" {
			m.view = ViewLoading
			m.loadingText = "Updating expiry..."
			return m, m.doUpdateExpiry(0)
		}
		t, err := time.Parse("2006-01-02", input)
		if err != nil {
			ts, err2 := strconv.ParseInt(input, 10, 64)
			if err2 != nil {
				m.modalInput = ""
				return m, nil
			}
			m.view = ViewLoading
			m.loadingText = "Updating expiry..."
			return m, m.doUpdateExpiry(ts)
		}
		m.view = ViewLoading
		m.loadingText = "Updating expiry..."
		return m, m.doUpdateExpiry(t.Unix())

	case ModalNewSession:
		label := input
		if label == "" {
			label = "lnconto-session"
		}
		m.view = ViewLoading
		m.loadingText = "Creating session..."
		return m, m.doCreateSession(label)
	}

	return m, nil
}

func (m *Model) viewModal() string {
	var content string

	switch m.modal {
	case ModalCredit:
		content = m.viewTextInputModal("Credit Account", "Amount in satoshis:", "e.g. 100000")
	case ModalDebit:
		content = m.viewTextInputModal("Debit Account", "Amount in satoshis:", "e.g. 50000")
	case ModalExpiry:
		content = m.viewTextInputModal("Set Expiry", "Date (YYYY-MM-DD) or 0 for never:", "e.g. 2025-12-31")
	case ModalNewSession:
		content = m.viewTextInputModal("New LNC Session", "Session label:", "e.g. My Wallet")
	case ModalMacaroonType:
		content = m.viewMacaroonTypeModal()
	case ModalMacaroonResult:
		content = m.viewResultModal()
	}

	return lipgloss.NewStyle().
		Width(m.safeWidth()).
		Height(m.safeHeight()).
		Align(lipgloss.Center, lipgloss.Center).
		Render(content)
}

func (m *Model) modalInnerWidth() int {
	// Modal box: border(2) + padding(6) = 8 overhead. Leave 4 margin each side.
	w := m.safeWidth() - 16
	if w < 30 {
		w = 30
	}
	if w > 60 {
		w = 60
	}
	return w
}

func (m *Model) viewTextInputModal(title, prompt, placeholder string) string {
	innerW := m.modalInnerWidth()

	inputDisplay := m.modalInput
	if inputDisplay == "" {
		inputDisplay = styleMuted.Render(placeholder)
	}

	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(0, 1).
		Width(innerW).
		Render(inputDisplay + "█")

	body := fmt.Sprintf(
		"%s\n\n%s\n%s\n\n%s",
		styleHeader.Render(title),
		styleLabel.Render(prompt),
		inputBox,
		styleHelp.Render("enter confirm   esc cancel   ctrl+u clear"),
	)
	return styleModal.Width(innerW).Render(body)
}

func (m *Model) viewMacaroonTypeModal() string {
	innerW := m.modalInnerWidth()
	body := fmt.Sprintf(
		"%s\n\n%s\n%s\n%s\n\n%s",
		styleHeader.Render("Generate Macaroon"),
		styleValue.Render("1")+" "+styleLabel.Render("Account       (invoices + offchain r/w)"),
		styleValue.Render("2")+" "+styleLabel.Render("Readonly      (invoices + offchain read)"),
		styleValue.Render("3")+" "+styleLabel.Render("Invoice       (create/read invoices only)"),
		styleHelp.Render("1-3 select   esc cancel"),
	)
	return styleModal.Width(innerW).Render(body)
}

func (m *Model) viewResultModal() string {
	// Make the result modal wider since it may contain a long macaroon hex.
	innerW := m.safeWidth() - 8
	if innerW < 30 {
		innerW = 30
	}

	// Wrap the result body to the inner content width.
	// styleModal has Padding(1,3) → content width = innerW - 6.
	contentW := innerW - 6
	if contentW < 20 {
		contentW = 20
	}
	wrapped := wrapText(m.modalResult, contentW)

	body := fmt.Sprintf("%s\n\n%s\n\n%s",
		styleHeader.Render("Result"),
		styleValue.Render(wrapped),
		styleHelp.Render("any key to close"),
	)
	return styleModal.Width(innerW).Render(body)
}
