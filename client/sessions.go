package client

import (
	"context"
	"fmt"

	"github.com/lightninglabs/lightning-terminal/litrpc"
)

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
