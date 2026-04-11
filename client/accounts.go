package client

import (
	"context"
	"fmt"

	"github.com/lightninglabs/lightning-terminal/litrpc"
)

// ListAccounts returns all accounts.
func (c *Client) ListAccounts(ctx context.Context) ([]*litrpc.Account, error) {
	resp, err := c.Accounts.ListAccounts(ctx, &litrpc.ListAccountsRequest{})
	if err != nil {
		return nil, fmt.Errorf("ListAccounts: %w", err)
	}
	return resp.Accounts, nil
}

// GetAccount returns a single account by ID.
func (c *Client) GetAccount(ctx context.Context, id string) (*litrpc.Account, error) {
	resp, err := c.Accounts.AccountInfo(ctx, &litrpc.AccountInfoRequest{Id: id})
	if err != nil {
		return nil, fmt.Errorf("AccountInfo: %w", err)
	}
	return resp, nil
}

// CreditAccount adds sats to an account balance.
func (c *Client) CreditAccount(ctx context.Context, id string, amount uint64) (*litrpc.Account, error) {
	resp, err := c.Accounts.CreditAccount(ctx, &litrpc.CreditAccountRequest{
		Account: &litrpc.AccountIdentifier{
			Identifier: &litrpc.AccountIdentifier_Id{Id: id},
		},
		Amount: amount,
	})
	if err != nil {
		return nil, fmt.Errorf("CreditAccount: %w", err)
	}
	return resp.Account, nil
}

// DebitAccount removes sats from an account balance.
func (c *Client) DebitAccount(ctx context.Context, id string, amount uint64) (*litrpc.Account, error) {
	resp, err := c.Accounts.DebitAccount(ctx, &litrpc.DebitAccountRequest{
		Account: &litrpc.AccountIdentifier{
			Identifier: &litrpc.AccountIdentifier_Id{Id: id},
		},
		Amount: amount,
	})
	if err != nil {
		return nil, fmt.Errorf("DebitAccount: %w", err)
	}
	return resp.Account, nil
}

// UpdateExpiry sets a new expiration date (unix timestamp, 0 = never).
func (c *Client) UpdateExpiry(ctx context.Context, id string, expiryUnix int64) (*litrpc.Account, error) {
	resp, err := c.Accounts.UpdateAccount(ctx, &litrpc.UpdateAccountRequest{
		Id:             id,
		AccountBalance: -1, // do not change balance
		ExpirationDate: expiryUnix,
	})
	if err != nil {
		return nil, fmt.Errorf("UpdateAccount: %w", err)
	}
	return resp, nil
}

// CreateAccount creates a new account with the given balance, label, and
// expiry (unix timestamp, 0 = never).
func (c *Client) CreateAccount(ctx context.Context, balance uint64, label string, expiryUnix int64) (*litrpc.Account, error) {
	resp, err := c.Accounts.CreateAccount(ctx, &litrpc.CreateAccountRequest{
		AccountBalance: balance,
		Label:          label,
		ExpirationDate: expiryUnix,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateAccount: %w", err)
	}
	return resp.Account, nil
}

// RemoveAccount deletes an account by ID.
func (c *Client) RemoveAccount(ctx context.Context, id string) error {
	_, err := c.Accounts.RemoveAccount(ctx, &litrpc.RemoveAccountRequest{Id: id})
	if err != nil {
		return fmt.Errorf("RemoveAccount: %w", err)
	}
	return nil
}
