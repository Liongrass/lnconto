package ui

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lightninglabs/lightning-terminal/litrpc"
	"github.com/lnconto/lnconto/client"
)

// View represents which screen is currently shown.
type View int

const (
	ViewLoading View = iota
	ViewDashboard
	ViewAccountDetail
	ViewSessions
	ViewModal
	ViewError
	ViewPaymentDetail
)

// ModalKind identifies what modal is open.
type ModalKind int

const (
	ModalNone ModalKind = iota
	ModalCredit
	ModalDebit
	ModalExpiry
	ModalNewSession
	ModalNewSessionExpiry
	ModalMacaroonType
	ModalMacaroonResult
	ModalNewAccountLabel
	ModalNewAccountBalance
	ModalNewAccountExpiry
	ModalLabel
	ModalConfirmRemove
	ModalSessionDetail
	ModalSaveMacaroon
	ModalNewGeneralSessionType
	ModalNewGeneralSessionPerms
	ModalNewGeneralSessionLabel
	ModalNewGeneralSessionExpiry
)

// msgs for async operations
type msgNodeInfo struct{ info *client.NodeInfo }
type msgAccountList struct{ accounts []*litrpc.Account }
type msgSessionList struct{ sessions []*litrpc.Session }
type msgAccountUpdated struct{ account *litrpc.Account }
type msgAccountCreated struct{ account *litrpc.Account }
type msgAccountRemoved struct{ id string }
type msgSessionRevoked struct{ localPubKey []byte }
type msgSessionCreated struct{ session *litrpc.Session }
type msgMacaroon struct{ hex string }
type msgMacaroonSaved struct{ path string }
type msgError struct{ err error }
type msgLoading struct{ text string }
type msgCopied struct{}
type msgPaymentsLoaded struct {
	payments []*client.PaymentInfo
}

// Model is the root bubbletea model.
type Model struct {
	client *client.Client
	width  int
	height int

	view        View
	prevView    View
	err         error
	loadingText string

	nodeInfo *client.NodeInfo
	accounts []*litrpc.Account
	sessions []*litrpc.Session

	selectedAccount int
	selectedSession int

	// scroll offsets for each list view
	accountsScroll int
	sessionsScroll int
	paymentsScroll int

	// sessions view filter
	hideInactiveSessions bool

	// modal state
	modal             ModalKind
	modalInput        string
	modalTitle        string
	modalSessionLabel string // holds the label while the expiry step is shown

	// general session creation state (Sessions view → new session flow)
	modalGeneralSessionType  int    // 0=admin 1=readonly 2=invoice 3=custom
	modalGeneralSessionPerms string // stored custom permissions string

	// new-account multi-step state
	modalAccountLabel   string // label collected in step 1
	modalAccountBalance uint64 // balance collected in step 2

	// session detail modal state
	modalSessionLocalKey []byte // local public key of the session being viewed

	// clipboard / result modal state
	clipboardPayload      string // raw value to copy (macaroon hex or pairing phrase)
	copied                bool   // true after a successful OSC-52 copy
	modalResultIsMacaroon bool   // true when result is a macaroon (not a pairing phrase)
	macaroonSaved         bool   // true after a successful file save
	macaroonSavedPath     string // path that was last saved to

	// payment list for the current account
	enrichedPayments []*client.PaymentInfo
	paymentsLoading  bool
	selectedPayment  int
}

// New creates the initial model.
func New(c *client.Client) *Model {
	return &Model{
		client:      c,
		view:        ViewLoading,
		loadingText: "Connecting to litd...",
	}
}

func (m *Model) Init() tea.Cmd {
	return m.fetchAll()
}

// safeWidth returns the terminal width with a sensible fallback.
func (m *Model) safeWidth() int {
	if m.width <= 0 {
		return 80
	}
	return m.width
}

// safeHeight returns the terminal height with a sensible fallback.
func (m *Model) safeHeight() int {
	if m.height <= 0 {
		return 24
	}
	return m.height
}

// clampScroll adjusts scroll so that selected is always within the visible window.
func clampScroll(scroll, selected, visible int) int {
	if visible <= 0 {
		visible = 1
	}
	if selected < scroll {
		return selected
	}
	if selected >= scroll+visible {
		return selected - visible + 1
	}
	return scroll
}

func (m *Model) fetchAll() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		info, err := m.client.GetNodeInfo(ctx)
		if err != nil {
			return msgError{err}
		}
		return msgNodeInfo{info}
	}
}

func (m *Model) fetchAccounts() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		accounts, err := m.client.ListAccounts(ctx)
		if err != nil {
			return msgError{err}
		}
		return msgAccountList{accounts}
	}
}

func (m *Model) fetchSessions() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		sessions, err := m.client.ListSessions(ctx)
		if err != nil {
			return msgError{err}
		}
		return msgSessionList{sessions}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case msgNodeInfo:
		m.nodeInfo = msg.info
		m.loadingText = "Loading accounts..."
		return m, m.fetchAccounts()

	case msgAccountList:
		m.accounts = msg.accounts
		m.view = ViewDashboard
		return m, nil

	case msgSessionList:
		m.sessions = msg.sessions
		m.sessionsScroll = 0
		m.selectedSession = 0
		m.view = ViewSessions
		return m, nil

	case msgSessionRevoked:
		for i, s := range m.sessions {
			if string(s.LocalPublicKey) == string(msg.localPubKey) {
				m.sessions[i].SessionState = litrpc.SessionState_STATE_REVOKED
				break
			}
		}
		m.modal = ModalNone
		m.modalSessionLocalKey = nil
		m.clipboardPayload = ""
		m.copied = false
		// Keep selectedSession within the (potentially now-shorter) filtered list.
		filtered := m.filteredSessions()
		if m.selectedSession >= len(filtered) && m.selectedSession > 0 {
			m.selectedSession = len(filtered) - 1
		}
		m.sessionsScroll = clampScroll(m.sessionsScroll, m.selectedSession, m.visibleSessionRows())
		m.view = ViewSessions
		return m, nil

	case msgAccountUpdated:
		for i, a := range m.accounts {
			if a.Id == msg.account.Id {
				m.accounts[i] = msg.account
				break
			}
		}
		m.modal = ModalNone
		m.modalInput = ""
		m.paymentsLoading = true
		m.enrichedPayments = nil
		m.selectedPayment = 0
		m.paymentsScroll = 0
		acc := m.accounts[m.selectedAccount]
		m.view = ViewAccountDetail
		return m, m.doLoadPayments(acc.Id)

	case msgAccountCreated:
		m.accounts = append(m.accounts, msg.account)
		m.selectedAccount = len(m.accounts) - 1
		m.accountsScroll = clampScroll(m.accountsScroll, m.selectedAccount, m.visibleAccountRows())
		m.modal = ModalNone
		m.modalInput = ""
		m.modalAccountLabel = ""
		m.modalAccountBalance = 0
		m.view = ViewDashboard
		return m, nil

	case msgAccountRemoved:
		for i, a := range m.accounts {
			if a.Id == msg.id {
				m.accounts = append(m.accounts[:i], m.accounts[i+1:]...)
				break
			}
		}
		if m.selectedAccount >= len(m.accounts) && m.selectedAccount > 0 {
			m.selectedAccount = len(m.accounts) - 1
		}
		m.accountsScroll = clampScroll(m.accountsScroll, m.selectedAccount, m.visibleAccountRows())
		m.modal = ModalNone
		m.modalInput = ""
		m.view = ViewDashboard
		return m, nil

	case msgCopied:
		m.copied = true
		return m, nil

	case msgPaymentsLoaded:
		m.enrichedPayments = msg.payments
		m.paymentsLoading = false
		m.selectedPayment = 0
		m.paymentsScroll = 0
		return m, nil

	case msgSessionCreated:
		m.modal = ModalNone
		m.modalInput = ""
		m.copied = false
		m.macaroonSaved = false
		m.macaroonSavedPath = ""
		m.modalResultIsMacaroon = false
		m.clipboardPayload = msg.session.PairingSecretMnemonic
		m.modal = ModalMacaroonResult
		m.view = ViewModal
		return m, func() tea.Msg { return tea.DisableMouse() }

	case msgMacaroon:
		m.modal = ModalNone
		m.modalInput = ""
		m.copied = false
		m.macaroonSaved = false
		m.macaroonSavedPath = ""
		m.modalResultIsMacaroon = true
		m.clipboardPayload = msg.hex
		m.modal = ModalMacaroonResult
		m.view = ViewModal
		return m, func() tea.Msg { return tea.DisableMouse() }

	case msgMacaroonSaved:
		m.macaroonSaved = true
		m.macaroonSavedPath = msg.path
		m.modal = ModalMacaroonResult
		m.view = ViewModal
		return m, nil

	case msgError:
		m.err = msg.err
		m.view = ViewError
		return m, nil

	case msgLoading:
		m.loadingText = msg.text
		m.view = ViewLoading
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		// Quit from anywhere.
		if m.view == ViewModal && m.modal == ModalMacaroonResult {
			// Restore mouse before exiting so the terminal is left clean.
			return m, tea.Batch(
				func() tea.Msg { return tea.EnableMouseCellMotion() },
				tea.Quit,
			)
		}
		return m, tea.Quit
	case "esc":
		// Go back one level from anywhere.
		if m.view == ViewModal {
			if m.modal == ModalSaveMacaroon {
				// Step back within the modal stack.
				m.modal = ModalMacaroonResult
				m.modalInput = ""
				return m, nil
			}
			wasResult := m.modal == ModalMacaroonResult
			m.modal = ModalNone
			m.modalInput = ""
			m.view = m.prevView
			if wasResult {
				return m, func() tea.Msg { return tea.EnableMouseCellMotion() }
			}
			return m, nil
		}
		if m.view == ViewPaymentDetail {
			m.view = ViewAccountDetail
			return m, nil
		}
		if m.view == ViewAccountDetail || m.view == ViewSessions || m.view == ViewError {
			m.view = ViewDashboard
			return m, nil
		}
		return m, nil
	}

	switch m.view {
	case ViewDashboard:
		return m.handleDashboardKey(msg)
	case ViewAccountDetail:
		return m.handleAccountDetailKey(msg)
	case ViewSessions:
		return m.handleSessionsKey(msg)
	case ViewModal:
		return m.handleModalKey(msg)
	case ViewPaymentDetail:
		return m.handlePaymentDetailKey(msg)
	case ViewError:
		if msg.String() == "r" {
			m.err = nil
			m.view = ViewLoading
			m.loadingText = "Reconnecting..."
			return m, m.fetchAll()
		}
	}
	return m, nil
}

func (m *Model) View() string {
	switch m.view {
	case ViewLoading:
		return m.viewLoading()
	case ViewDashboard:
		return m.viewDashboard()
	case ViewAccountDetail:
		return m.viewAccountDetail()
	case ViewSessions:
		return m.viewSessions()
	case ViewModal:
		return m.viewModal()
	case ViewPaymentDetail:
		return m.viewPaymentDetail()
	case ViewError:
		return m.viewError()
	}
	return ""
}

func (m *Model) doCreateAccount(balance uint64, label string, expiryUnix int64) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		acc, err := m.client.CreateAccount(ctx, balance, label, expiryUnix)
		if err != nil {
			return msgError{err}
		}
		return msgAccountCreated{acc}
	}
}

func doSaveMacaroon(hexStr, path string) tea.Cmd {
	return func() tea.Msg {
		// Expand ~ to the user's home directory.
		if strings.HasPrefix(path, "~/") {
			home, err := os.UserHomeDir()
			if err == nil {
				path = filepath.Join(home, path[2:])
			}
		}
		data, err := hex.DecodeString(hexStr)
		if err != nil {
			return msgError{fmt.Errorf("invalid macaroon data: %w", err)}
		}
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return msgError{fmt.Errorf("creating directory: %w", err)}
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			return msgError{fmt.Errorf("saving macaroon: %w", err)}
		}
		return msgMacaroonSaved{path}
	}
}

func (m *Model) doCreateGeneralSession(sessionType litrpc.SessionType, customPerms []*litrpc.MacaroonPermission, label string, expiryUnix uint64) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		session, err := m.client.CreateGeneralSession(ctx, label, sessionType, customPerms, expiryUnix)
		if err != nil {
			return msgError{err}
		}
		return msgSessionCreated{session}
	}
}

func (m *Model) doLoadPayments(accountID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		payments, err := m.client.ListAccountPayments(ctx, accountID)
		if err != nil {
			return msgError{err}
		}
		return msgPaymentsLoaded{payments: payments}
	}
}

func (m *Model) doRevokeSession(localPubKey []byte) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if err := m.client.RevokeSession(ctx, localPubKey); err != nil {
			return msgError{err}
		}
		return msgSessionRevoked{localPubKey}
	}
}

func (m *Model) doRemoveAccount(id string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if err := m.client.RemoveAccount(ctx, id); err != nil {
			return msgError{err}
		}
		return msgAccountRemoved{id}
	}
}

func (m *Model) viewLoading() string {
	return lipgloss.NewStyle().
		Width(m.safeWidth()).
		Height(m.safeHeight()).
		Align(lipgloss.Center, lipgloss.Center).
		Render("⚡ " + m.loadingText)
}

func (m *Model) viewError() string {
	errMsg := "unknown error"
	if m.err != nil {
		errMsg = m.err.Error()
	}
	// Wrap long error messages to terminal width.
	innerW := m.safeWidth() - 10
	if innerW < 30 {
		innerW = 30
	}
	box := styleModal.Width(innerW).Render(
		styleHeader.Render("Connection Error") + "\n\n" +
			styleRed.Render(errMsg) + "\n\n" +
			styleHelp.Render("r retry   esc back"),
	)
	return lipgloss.NewStyle().
		Width(m.safeWidth()).
		Height(m.safeHeight()).
		Align(lipgloss.Center, lipgloss.Center).
		Render(box)
}
