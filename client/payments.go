package client

import (
	"context"
	"fmt"
	"sort"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/zpay32"
	"google.golang.org/grpc"
)

// PaymentInfo holds either an outgoing payment or an incoming invoice.
// Exactly one of Payment or Invoice is non-nil.
type PaymentInfo struct {
	Payment *lnrpc.Payment  // non-nil for outgoing payments
	Invoice *lnrpc.Invoice  // non-nil for incoming invoices
	Memo    string
}

// IsIncoming reports whether this is an incoming invoice.
func (pi *PaymentInfo) IsIncoming() bool { return pi.Invoice != nil }

// TimestampSec returns a unix-second timestamp suitable for sorting.
// For invoices: settle date if settled, otherwise creation date.
// For payments: creation time converted from nanoseconds.
func (pi *PaymentInfo) TimestampSec() int64 {
	if pi.Invoice != nil {
		if pi.Invoice.SettleDate > 0 {
			return pi.Invoice.SettleDate
		}
		return pi.Invoice.CreationDate
	}
	return pi.Payment.CreationTimeNs / 1_000_000_000
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

	lightning := lnrpc.NewLightningClient(conn)

	// Outgoing payments.
	payResp, err := lightning.ListPayments(ctx, &lnrpc.ListPaymentsRequest{
		Reversed:          true,
		IncludeIncomplete: true,
	})
	if err != nil {
		return nil, fmt.Errorf("ListPayments: %w", err)
	}

	// Incoming invoices.
	invResp, err := lightning.ListInvoices(ctx, &lnrpc.ListInvoiceRequest{
		Reversed: true,
	})
	if err != nil {
		return nil, fmt.Errorf("ListInvoices: %w", err)
	}

	var result []*PaymentInfo

	for _, p := range payResp.Payments {
		result = append(result, &PaymentInfo{
			Payment: p,
			Memo:    decodeMemo(p.PaymentRequest, c.Network),
		})
	}

	for _, inv := range invResp.Invoices {
		memo := inv.Memo
		if memo == "" {
			memo = decodeMemo(inv.PaymentRequest, c.Network)
		}
		result = append(result, &PaymentInfo{
			Invoice: inv,
			Memo:    memo,
		})
	}

	// Sort newest first.
	sort.Slice(result, func(i, j int) bool {
		return result[i].TimestampSec() > result[j].TimestampSec()
	})

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
