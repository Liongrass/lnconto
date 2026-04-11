package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/lightninglabs/lightning-terminal/litrpc"
)

// InvoiceSessionPermissions are the predefined permissions for the invoice
// session type: create and read invoices, read addresses and on-chain data.
var InvoiceSessionPermissions = []*litrpc.MacaroonPermission{
	{Entity: "address", Action: "read"},
	{Entity: "address", Action: "write"},
	{Entity: "invoices", Action: "read"},
	{Entity: "invoices", Action: "write"},
	{Entity: "onchain", Action: "read"},
}

// ParsePermissions splits a comma-separated "entity:action" string into a
// slice of MacaroonPermissions.  Returns an error if any entry is malformed.
func ParsePermissions(s string) ([]*litrpc.MacaroonPermission, error) {
	var perms []*litrpc.MacaroonPermission
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 || kv[0] == "" || kv[1] == "" {
			return nil, fmt.Errorf("invalid permission %q: want entity:action", part)
		}
		perms = append(perms, &litrpc.MacaroonPermission{
			Entity: strings.TrimSpace(kv[0]),
			Action: strings.TrimSpace(kv[1]),
		})
	}
	if len(perms) == 0 {
		return nil, fmt.Errorf("no permissions provided")
	}
	return perms, nil
}

// ListSessions returns all sessions.
func (c *Client) ListSessions(ctx context.Context) ([]*litrpc.Session, error) {
	resp, err := c.Sessions.ListSessions(ctx, &litrpc.ListSessionsRequest{})
	if err != nil {
		return nil, fmt.Errorf("ListSessions: %w", err)
	}
	return resp.Sessions, nil
}

// RevokeSession revokes an active session identified by its local public key.
func (c *Client) RevokeSession(ctx context.Context, localPubKey []byte) error {
	_, err := c.Sessions.RevokeSession(ctx, &litrpc.RevokeSessionRequest{
		LocalPublicKey: localPubKey,
	})
	if err != nil {
		return fmt.Errorf("RevokeSession: %w", err)
	}
	return nil
}

// CreateGeneralSession creates a new LNC session not tied to any account.
// For TYPE_MACAROON_CUSTOM, supply the desired permissions in customPerms.
func (c *Client) CreateGeneralSession(ctx context.Context, label string, sessionType litrpc.SessionType, customPerms []*litrpc.MacaroonPermission, expiryUnix uint64) (*litrpc.Session, error) {
	if label == "" {
		label = "lnconto-session"
	}
	if expiryUnix == 0 {
		expiryUnix = uint64(86400 * 365) // 1 year from now handled by caller
	}
	req := &litrpc.AddSessionRequest{
		Label:                     label,
		SessionType:               sessionType,
		MailboxServerAddr:         "mailbox.terminal.lightning.today:443",
		ExpiryTimestampSeconds:    expiryUnix,
		MacaroonCustomPermissions: customPerms,
	}
	resp, err := c.Sessions.AddSession(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("AddSession: %w", err)
	}
	return resp.Session, nil
}

// CreateAccountSession creates a new LNC session tied to an account.
func (c *Client) CreateAccountSession(ctx context.Context, label, accountID, mailboxAddr string, expiryUnix uint64) (*litrpc.Session, error) {
	if mailboxAddr == "" {
		mailboxAddr = "mailbox.terminal.lightning.today:443"
	}
	resp, err := c.Sessions.AddSession(ctx, &litrpc.AddSessionRequest{
		Label:                  label,
		SessionType:            litrpc.SessionType_TYPE_MACAROON_ACCOUNT,
		AccountId:              accountID,
		MailboxServerAddr:      mailboxAddr,
		ExpiryTimestampSeconds: expiryUnix,
	})
	if err != nil {
		return nil, fmt.Errorf("AddSession: %w", err)
	}
	return resp.Session, nil
}
