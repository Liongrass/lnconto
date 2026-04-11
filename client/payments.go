package client

import (
	"context"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/zpay32"
	"google.golang.org/grpc"
)

// PaymentInfo wraps an lnrpc.Payment with a pre-decoded memo.
type PaymentInfo struct {
	Payment *lnrpc.Payment
	Memo    string
}

// ListAccountPayments bakes a readonly account-attenuated macaroon and opens a
// dedicated gRPC connection that carries only that macaroon, so the litd
// middleware sees exactly one credential and filters ListPayments to the
// payments belonging to this account. Results are returned newest-first.
func (c *Client) ListAccountPayments(ctx context.Context, accountID string) ([]*PaymentInfo, error) {
	macHex, err := c.BakeAccountMacaroon(ctx, accountID, MacaroonTypeAccountReadonly)
	if err != nil {
		return nil, fmt.Errorf("baking account macaroon: %w", err)
	}

	// Open a short-lived connection with only the account macaroon.
	// Using the existing supermacaroon connection would send two macaroons.
	creds := &macaroonCredentials{hex: macHex}
	conn, err := grpc.NewClient(
		c.rpcServer,
		grpc.WithTransportCredentials(c.tlsCreds),
		grpc.WithPerRPCCredentials(creds),
	)
	if err != nil {
		return nil, fmt.Errorf("dialing for account payments: %w", err)
	}
	defer conn.Close()

	resp, err := lnrpc.NewLightningClient(conn).ListPayments(ctx, &lnrpc.ListPaymentsRequest{
		Reversed:          true,
		IncludeIncomplete: true,
	})
	if err != nil {
		return nil, fmt.Errorf("ListPayments: %w", err)
	}

	result := make([]*PaymentInfo, len(resp.Payments))
	for i, p := range resp.Payments {
		result[i] = &PaymentInfo{
			Payment: p,
			Memo:    decodeMemo(p.PaymentRequest, c.Network),
		}
	}
	return result, nil
}

// decodeMemo extracts the description from a bolt11 payment request string.
// Returns "" if the request is empty, cannot be decoded, or has no description.
func decodeMemo(payReq, network string) string {
	if payReq == "" {
		return ""
	}
	params := netParams(network)
	inv, err := zpay32.Decode(payReq, params)
	if err != nil || inv.Description == nil {
		return ""
	}
	return *inv.Description
}

func netParams(network string) *chaincfg.Params {
	switch network {
	case "testnet", "testnet3":
		return &chaincfg.TestNet3Params
	case "signet":
		return &chaincfg.SigNetParams
	case "regtest", "simnet":
		return &chaincfg.RegressionNetParams
	default:
		return &chaincfg.MainNetParams
	}
}
