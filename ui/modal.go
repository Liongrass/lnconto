package ui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lightninglabs/lightning-terminal/litrpc"
	"github.com/lnconto/lnconto/client"
)

func (m *Model) handleModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.modal {
	case ModalCredit, ModalDebit, ModalExpiry, ModalNewSession, ModalNewSessionExpiry,
		ModalNewAccountLabel, ModalNewAccountBalance, ModalNewAccountExpiry,
		ModalSaveMacaroon:
		return m.handleTextInputKey(msg)
	case ModalConfirmRemove:
		return m.handleConfirmRemoveKey(msg)
	case ModalSessionDetail:
		return m.handleSessionDetailKey(msg)
	case ModalMacaroonType:
		return m.handleMacaroonTypeKey(msg)
	case ModalMacaroonResult:
		switch msg.String() {
		case "c":
			return m, copyToClipboard(m.clipboardPayload)
		case "s":
			if m.modalResultIsMacaroon {
				m.modal = ModalSaveMacaroon
				m.modalInput = ""
				return m, nil
			}
		}
		// Any other key closes the modal and restores mouse tracking.
		m.modal = ModalNone
		m.modalResult = ""
		m.clipboardPayload = ""
		m.copied = false
		m.macaroonSaved = false
		m.macaroonSavedPath = ""
		m.view = m.prevView
		return m, func() tea.Msg { return tea.EnableMouseCellMotion() }
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

func (m *Model) handleSessionDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "c":
		// Only copy if the session hasn't been connected to yet.
		if m.clipboardPayload != "" && len(m.modalSessionLocalKey) > 0 {
			var connected bool
			for _, s := range m.sessions {
				if string(s.LocalPublicKey) == string(m.modalSessionLocalKey) {
					connected = len(s.RemotePublicKey) > 0
					break
				}
			}
			if !connected {
				return m, copyToClipboard(m.clipboardPayload)
			}
		}
	case "r":
		if m.modalSessionLocalKey != nil {
			key := m.modalSessionLocalKey
			m.modal = ModalNone
			m.modalSessionLocalKey = nil
			m.clipboardPayload = ""
			m.copied = false
			m.view = ViewLoading
			m.loadingText = "Revoking session..."
			return m, m.doRevokeSession(key)
		}
	}
	// Any other key closes the modal.
	m.modal = ModalNone
	m.modalSessionLocalKey = nil
	m.clipboardPayload = ""
	m.copied = false
	m.view = m.prevView
	return m, nil
}

func (m *Model) handleConfirmRemoveKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "y" || msg.String() == "Y" {
		acc := m.selectedAcc()
		if acc == nil {
			m.modal = ModalNone
			m.view = m.prevView
			return m, nil
		}
		id := acc.Id
		m.modal = ModalNone
		m.view = ViewLoading
		m.loadingText = "Removing account..."
		return m, m.doRemoveAccount(id)
	}
	// Any other key cancels.
	m.modal = ModalNone
	m.view = m.prevView
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
		// Advance to the expiry step.
		m.modalSessionLabel = label
		m.modalInput = ""
		m.modal = ModalNewSessionExpiry
		return m, nil

	case ModalNewSessionExpiry:
		expiry, err := parseSessionExpiry(input)
		if err != nil {
			m.modalInput = ""
			return m, nil
		}
		m.view = ViewLoading
		m.loadingText = "Creating session..."
		return m, m.doCreateSession(m.modalSessionLabel, expiry)

	case ModalNewAccountLabel:
		// Label is optional; advance to balance step.
		m.modalAccountLabel = input
		m.modalInput = ""
		m.modal = ModalNewAccountBalance
		return m, nil

	case ModalNewAccountBalance:
		if input == "" {
			return m, nil
		}
		amount, err := strconv.ParseUint(input, 10, 64)
		if err != nil {
			m.modalInput = ""
			return m, nil
		}
		m.modalAccountBalance = amount
		m.modalInput = ""
		m.modal = ModalNewAccountExpiry
		return m, nil

	case ModalNewAccountExpiry:
		expiry, err := parseAccountExpiry(input)
		if err != nil {
			m.modalInput = ""
			return m, nil
		}
		m.view = ViewLoading
		m.loadingText = "Creating account..."
		return m, m.doCreateAccount(m.modalAccountBalance, m.modalAccountLabel, expiry)

	case ModalSaveMacaroon:
		if input == "" {
			return m, nil
		}
		hex := m.clipboardPayload
		m.modal = ModalMacaroonResult
		m.modalInput = ""
		return m, doSaveMacaroon(hex, input)
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
	case ModalNewSessionExpiry:
		content = m.viewTextInputModal(
			fmt.Sprintf(`Session expiry for "%s"`, m.modalSessionLabel),
			"Duration (leave blank for 1 year):",
			"30m  2h  7d  3mo  1y",
		)
	case ModalNewAccountLabel:
		content = m.viewTextInputModal("New Account", "Label (optional):", "e.g. My Budget")
	case ModalNewAccountBalance:
		title := "New Account"
		if m.modalAccountLabel != "" {
			title = fmt.Sprintf(`New Account "%s"`, m.modalAccountLabel)
		}
		content = m.viewTextInputModal(title, "Initial balance (satoshis):", "e.g. 100000  (0 for empty)")
	case ModalNewAccountExpiry:
		content = m.viewTextInputModal(
			"New Account — Expiry",
			"Expiry (blank = never):",
			"30m  2h  7d  3mo  1y  or YYYY-MM-DD",
		)
	case ModalSaveMacaroon:
		content = m.viewTextInputModal("Save Macaroon", "File path:", "~/wallet.macaroon")
	case ModalConfirmRemove:
		content = m.viewConfirmRemoveModal()
	case ModalSessionDetail:
		content = m.viewSessionDetailModal()
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

// modalInnerWidth returns the content width for compact input/choice modals.
// Capped at 60 so they don't sprawl across a wide terminal.
func (m *Model) modalInnerWidth() int {
	w := m.safeWidth() - 16
	if w < 30 {
		w = 30
	}
	if w > 60 {
		w = 60
	}
	return w
}

// contentModalWidth returns the content width for result/detail modals that
// benefit from filling more of the terminal (pairing phrases, macaroon hex).
func (m *Model) contentModalWidth() int {
	w := m.safeWidth() - 8
	if w < 30 {
		w = 30
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
	innerW := m.contentModalWidth()

	// Wrap the result body to the inner content width.
	// styleModal has Padding(1,3) → content width = innerW - 6.
	contentW := innerW - 6
	if contentW < 20 {
		contentW = 20
	}
	wrapped := wrapText(m.modalResult, contentW)

	var statusLine string
	switch {
	case m.copied:
		statusLine = styleGreen.Render("✓ Copied to clipboard!")
	case m.macaroonSaved:
		statusLine = styleGreen.Render("✓ Saved to " + m.macaroonSavedPath)
	default:
		statusLine = styleHelp.Render("select text to copy   c clipboard")
	}

	var helpLine string
	if m.modalResultIsMacaroon {
		helpLine = styleHelp.Render("s save to file   any other key to close")
	} else {
		helpLine = styleHelp.Render("any other key to close")
	}

	body := fmt.Sprintf("%s\n\n%s\n\n%s\n%s",
		styleHeader.Render("Result"),
		styleValue.Render(wrapped),
		statusLine,
		helpLine,
	)
	return styleModal.Width(innerW).Render(body)
}

func (m *Model) viewSessionDetailModal() string {
	innerW := m.contentModalWidth()

	// Find the session by local public key.
	var s *litrpc.Session
	for _, sess := range m.sessions {
		if string(sess.LocalPublicKey) == string(m.modalSessionLocalKey) {
			s = sess
			break
		}
	}
	if s == nil {
		return styleModal.Width(innerW).Render(styleRed.Render("Session not found."))
	}

	stateStyle := styleNormal
	switch s.SessionState {
	case litrpc.SessionState_STATE_IN_USE:
		stateStyle = styleGreen
	case litrpc.SessionState_STATE_REVOKED, litrpc.SessionState_STATE_EXPIRED:
		stateStyle = styleMuted
	}

	details := fmt.Sprintf(
		"%s %s\n%s %s\n%s %s\n%s %s",
		styleLabel.Render("Label:  "), styleValue.Render(s.Label),
		styleLabel.Render("Type:   "), styleValue.Render(sessionTypeName(s.SessionType)),
		styleLabel.Render("State:  "), stateStyle.Render(sessionStateName(s.SessionState)),
		styleLabel.Render("Expiry: "), styleValue.Render(formatExpiry(int64(s.ExpiryTimestampSeconds))),
	)
	if s.AccountId != "" {
		details += "\n" + styleLabel.Render("Account:") + " " + styleValue.Render(truncate(s.AccountId, innerW-10))
	}

	// Show pairing phrase only when the session has not been connected to yet
	// (remote_public_key unset). Once a wallet has paired, show "session in use"
	// instead — the phrase is no longer needed and showing it would be confusing.
	sessionConnected := len(s.RemotePublicKey) > 0
	var phraseSection string
	if !sessionConnected && s.PairingSecretMnemonic != "" {
		contentW := innerW - 6
		if contentW < 20 {
			contentW = 20
		}
		phraseSection = "\n\n" + styleLabel.Render("Pairing phrase:") + "\n" +
			styleValue.Render(wrapText(s.PairingSecretMnemonic, contentW))
	} else if sessionConnected {
		phraseSection = "\n\n" + styleGreen.Render("● session in use")
	}

	// Build help line based on available actions.
	canRevoke := s.SessionState == litrpc.SessionState_STATE_CREATED ||
		s.SessionState == litrpc.SessionState_STATE_IN_USE
	var helpParts []string
	if !sessionConnected && s.PairingSecretMnemonic != "" {
		if m.copied {
			helpParts = append(helpParts, styleGreen.Render("✓ copied"))
		} else {
			helpParts = append(helpParts, "c copy phrase")
		}
	}
	if canRevoke {
		helpParts = append(helpParts, "r revoke")
	}
	helpParts = append(helpParts, "any other key close")

	body := fmt.Sprintf("%s\n\n%s%s\n\n%s",
		styleHeader.Render("Session Detail"),
		details,
		phraseSection,
		styleHelp.Render(strings.Join(helpParts, "   ")),
	)
	return styleModal.Width(innerW).Render(body)
}

func (m *Model) viewConfirmRemoveModal() string {
	innerW := m.modalInnerWidth()
	body := fmt.Sprintf(
		"%s\n\n%s\n\n%s",
		styleHeader.Render("Remove Account"),
		styleValue.Render(`Remove "`+m.modalTitle+`"?`)+"\n"+
			styleWarning.Render("This cannot be undone."),
		styleHelp.Render("y confirm   any other key cancel"),
	)
	return styleModal.Width(innerW).Render(body)
}

// copyToClipboard writes the OSC 52 terminal sequence to stdout, which causes
// the terminal emulator to place text in the system clipboard.  This works in
// all OSC 52-capable terminals (kitty, Alacritty, WezTerm, iTerm2, GNOME
// Terminal ≥ 3.38, Foot, Windows Terminal, …).
func copyToClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		// ansi.SetSystemClipboard returns the OSC 52 escape sequence.
		// Writing it to stdout is safe: the sequence is invisible and the
		// terminal processes it independently of the rendered TUI output.
		os.Stdout.WriteString(xansi.SetSystemClipboard(text))
		return msgCopied{}
	}
}

// parseSessionExpiry converts a human-friendly duration string into a Unix
// timestamp in the future.  Accepted suffixes: m (minutes), h (hours),
// d (days), mo (months = 30 days), y (years = 365 days).  A bare integer is
// treated as days.  An empty string defaults to 1 year.
func parseSessionExpiry(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return uint64(time.Now().Add(365 * 24 * time.Hour).Unix()), nil
	}

	// Split into numeric part and suffix.
	i := 0
	for i < len(s) && (s[i] >= '0' && s[i] <= '9') {
		i++
	}
	if i == 0 {
		return 0, fmt.Errorf("invalid expiry %q", s)
	}
	n, err := strconv.ParseUint(s[:i], 10, 64)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("invalid expiry %q", s)
	}
	suffix := strings.ToLower(strings.TrimSpace(s[i:]))

	var d time.Duration
	switch suffix {
	case "", "d", "day", "days":
		d = time.Duration(n) * 24 * time.Hour
	case "m", "min", "mins", "minute", "minutes":
		d = time.Duration(n) * time.Minute
	case "h", "hr", "hrs", "hour", "hours":
		d = time.Duration(n) * time.Hour
	case "mo", "mon", "month", "months":
		d = time.Duration(n) * 30 * 24 * time.Hour
	case "y", "yr", "year", "years":
		d = time.Duration(n) * 365 * 24 * time.Hour
	default:
		return 0, fmt.Errorf("unknown unit %q (use m, h, d, mo, y)", suffix)
	}

	return uint64(time.Now().Add(d).Unix()), nil
}

// parseAccountExpiry parses an account expiry string.  Blank or "0" means
// never (returns 0).  Accepts YYYY-MM-DD dates and the same duration suffixes
// as parseSessionExpiry (m, h, d, mo, y).
func parseAccountExpiry(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return 0, nil
	}

	// Try YYYY-MM-DD first.
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.Unix(), nil
	}

	// Try duration suffixes (reuse session expiry logic, cast result).
	ts, err := parseSessionExpiry(s)
	if err != nil {
		return 0, err
	}
	return int64(ts), nil
}
