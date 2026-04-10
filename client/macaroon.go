package client

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/lightningnetwork/lnd/lnrpc"
	"gopkg.in/macaroon.v2"
)

// MacaroonType describes what permissions to bake into the macaroon.
type MacaroonType int

const (
	MacaroonTypeAccount        MacaroonType = iota // invoices read/write + offchain read/write + info read
	MacaroonTypeAccountReadonly                    // invoices read + offchain read + info read
	MacaroonTypeInvoice                            // invoices read/write only
)

// accountCaveat is the first-party caveat format LiT uses to tie a macaroon to an account.
func accountCaveat(accountID string) string {
	return "lnd-custom account " + accountID
}

// permissionsForType returns the LND MacaroonPermissions for a given type.
func permissionsForType(t MacaroonType) []*lnrpc.MacaroonPermission {
	switch t {
	case MacaroonTypeAccountReadonly:
		return []*lnrpc.MacaroonPermission{
			{Entity: "info", Action: "read"},
			{Entity: "invoices", Action: "read"},
			{Entity: "offchain", Action: "read"},
			{Entity: "onchain", Action: "read"},
		}
	case MacaroonTypeInvoice:
		return []*lnrpc.MacaroonPermission{
			{Entity: "invoices", Action: "read"},
			{Entity: "invoices", Action: "write"},
			{Entity: "address", Action: "read"},
			{Entity: "address", Action: "write"},
		}
	default: // MacaroonTypeAccount
		return []*lnrpc.MacaroonPermission{
			{Entity: "info", Action: "read"},
			{Entity: "invoices", Action: "read"},
			{Entity: "invoices", Action: "write"},
			{Entity: "offchain", Action: "read"},
			{Entity: "offchain", Action: "write"},
			{Entity: "onchain", Action: "read"},
			{Entity: "address", Action: "read"},
			{Entity: "address", Action: "write"},
		}
	}
}

// BakeAccountMacaroon bakes a macaroon with the given type, constrained to accountID.
// Returns the macaroon as a hex string.
func (c *Client) BakeAccountMacaroon(ctx context.Context, accountID string, t MacaroonType) (string, error) {
	perms := permissionsForType(t)
	resp, err := c.Lightning.BakeMacaroon(ctx, &lnrpc.BakeMacaroonRequest{
		Permissions:              perms,
		AllowExternalPermissions: true,
	})
	if err != nil {
		return "", fmt.Errorf("BakeMacaroon: %w", err)
	}

	// Decode the hex macaroon.
	macBytes, err := hex.DecodeString(resp.Macaroon)
	if err != nil {
		return "", fmt.Errorf("decoding macaroon hex: %w", err)
	}

	mac, err := macaroon.New(nil, nil, "", macaroon.LatestVersion)
	if err != nil {
		return "", fmt.Errorf("creating macaroon: %w", err)
	}
	if err := mac.UnmarshalBinary(macBytes); err != nil {
		return "", fmt.Errorf("unmarshalling macaroon: %w", err)
	}

	// Add account caveat.
	if err := mac.AddFirstPartyCaveat([]byte(accountCaveat(accountID))); err != nil {
		return "", fmt.Errorf("adding account caveat: %w", err)
	}

	constrained, err := mac.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("marshalling macaroon: %w", err)
	}

	return hex.EncodeToString(constrained), nil
}
