package ui

import (
	"context"
	"fmt"

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
)

// ModalKind identifies what modal is open.
type ModalKind int

const (
	ModalNone ModalKind = iota
	ModalCredit
	ModalDebit
	ModalExpiry
	ModalNewSession
	ModalMacaroonType
	ModalMacaroonResult
)

// msgs for async operations
type msgNodeInfo struct{ info *client.NodeInfo }
type msgAccountList struct{ accounts []*litrpc.Account }
type msgSessionList struct{ sessions []*litrpc.Session }
type msgAccountUpdated struct{ account *litrpc.Account }
type msgSessionCreated struct{ session *litrpc.Session }
type msgMacaroon struct{ hex string }
type msgError struct{ err error }
type msgLoading struct{ text string }

// Model is the root bubbletea model.
type Model struct {
	client  *client.Client
	width   int
	height  int

	view        View
	prevView    View
	err         error
	loadingText string

	nodeInfo *client.NodeInfo
	accounts []*litrpc.Account
	sessions []*litrpc.Session

	selectedAccount int
	selectedSession int

	// modal state
	modal       ModalKind
	modalInput  string
	modalResult string
	modalTitle  string

	// input cursor position for modal
	inputCursor int
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
		m.view = ViewSessions
		return m, nil

	case msgAccountUpdated:
		// Update the account in our list.
		for i, a := range m.accounts {
			if a.Id == msg.account.Id {
				m.accounts[i] = msg.account
				break
			}
		}
		if m.selectedAccount < len(m.accounts) {
			m.accounts[m.selectedAccount] = msg.account
		}
		m.modal = ModalNone
		m.modalInput = ""
		m.view = ViewAccountDetail
		return m, nil

	case msgSessionCreated:
		m.modal = ModalNone
		m.modalInput = ""
		m.modalResult = fmt.Sprintf(
			"Session created!\n\nPairing phrase:\n%s\n\nCopy this and use it in your LNC-compatible wallet.",
			msg.session.PairingSecretMnemonic,
		)
		m.modal = ModalMacaroonResult
		m.view = ViewModal
		return m, nil

	case msgMacaroon:
		m.modal = ModalNone
		m.modalInput = ""
		m.modalResult = fmt.Sprintf(
			"Macaroon (hex):\n\n%s\n\nStore this securely — it grants access to the account.",
			msg.hex,
		)
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
	// Global keys.
	switch msg.String() {
	case "ctrl+c", "q":
		if m.view == ViewModal {
			m.modal = ModalNone
			m.modalInput = ""
			m.view = m.prevView
			return m, nil
		}
		if m.view == ViewAccountDetail || m.view == ViewSessions {
			m.view = ViewDashboard
			return m, nil
		}
		return m, tea.Quit
	case "esc":
		if m.view == ViewModal {
			m.modal = ModalNone
			m.modalInput = ""
			m.view = m.prevView
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
	case ViewError:
		return m.viewError()
	}
	return ""
}

func (m *Model) viewLoading() string {
	content := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Render("⚡ " + m.loadingText)
	return content
}

func (m *Model) viewError() string {
	errMsg := "unknown error"
	if m.err != nil {
		errMsg = m.err.Error()
	}
	box := styleModal.Render(
		styleHeader.Render("Connection Error") + "\n\n" +
			styleRed.Render(errMsg) + "\n\n" +
			styleHelp.Render("r - retry   q - quit"),
	)
	return lipgloss.NewStyle().
		Width(m.width).Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(box)
}
