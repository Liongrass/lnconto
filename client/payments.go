package client

import (
	"context"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lightninglabs/lightning-terminal/litrpc"
	"github.com/lightningnetwork/lnd/lnrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PaymentDirection indicates whether a payment was incoming or outgoing.
type PaymentDirection int

const (
	DirectionIncoming PaymentDirection = iota
	DirectionOutgoing
	DirectionUnknown
)

// PaymentInfo is a unified view of a payment regardless of direction.
type PaymentInfo struct {
	Direction   PaymentDirection
	Hash        []byte
	HashHex     string
	AmountSat   int64
	FeeSat      int64
	TimestampNs int64 // nanoseconds since unix epoch
	Memo        string
	Status      string

	// At most one of these is non-nil.
	Invoice *lnrpc.Invoice
	Payment *lnrpc.Payment
}

// TimeUnix returns the payment timestamp as seconds since the unix epoch.
func (pi *PaymentInfo) TimeUnix() int64 {
	return pi.TimestampNs / 1_000_000_000
}

// ListPayments returns all payments with creation date ≥ sinceUnix (seconds).
func (c *Client) ListPayments(ctx context.Context, sinceUnix uint64) ([]*lnrpc.Payment, error) {
	resp, err := c.Lightning.ListPayments(ctx, &lnrpc.ListPaymentsRequest{
		IncludeIncomplete: true,
		Reversed:          true,
		CreationDateStart: sinceUnix,
	})
	if err != nil {
		return nil, fmt.Errorf("ListPayments: %w", err)
	}
	return resp.Payments, nil
}

// ListPaymentsRange returns payments with creation date in [startUnix, endUnix].
func (c *Client) ListPaymentsRange(ctx context.Context, startUnix, endUnix uint64) ([]*lnrpc.Payment, error) {
	resp, err := c.Lightning.ListPayments(ctx, &lnrpc.ListPaymentsRequest{
		IncludeIncomplete: true,
		Reversed:          true,
		CreationDateStart: startUnix,
		CreationDateEnd:   endUnix,
	})
	if err != nil {
		return nil, fmt.Errorf("ListPayments: %w", err)
	}
	return resp.Payments, nil
}

// LookupInvoice retrieves an invoice by payment hash.
// Returns (nil, nil) when the invoice does not exist.
func (c *Client) LookupInvoice(ctx context.Context, rHash []byte) (*lnrpc.Invoice, error) {
	inv, err := c.Lightning.LookupInvoice(ctx, &lnrpc.PaymentHash{RHash: rHash})
	if err != nil {
		st, ok := status.FromError(err)
		if ok && (st.Code() == codes.NotFound ||
			strings.Contains(st.Message(), "unable to locate invoice")) {
			return nil, nil
		}
		return nil, fmt.Errorf("LookupInvoice: %w", err)
	}
	return inv, nil
}

// EnrichAccountPayments resolves each account payment to an incoming invoice
// or outgoing payment from the cache. Payments not found in either source are
// returned as DirectionUnknown with whatever data litrpc provided.
// The result is sorted newest-first.
func (c *Client) EnrichAccountPayments(
	ctx context.Context,
	accPayments []*litrpc.AccountPayment,
	outgoingCache []*lnrpc.Payment,
) []*PaymentInfo {
	// Build an outgoing cache index keyed by hex payment hash.
	outIdx := make(map[string]*lnrpc.Payment, len(outgoingCache))
	for _, p := range outgoingCache {
		outIdx[p.PaymentHash] = p
	}

	result := make([]*PaymentInfo, 0, len(accPayments))
	for _, ap := range accPayments {
		result = append(result, enrichSingle(ctx, c, ap, outIdx))
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].TimestampNs > result[j].TimestampNs
	})
	return result
}

// ReEnrichUnknown re-attempts enrichment for any DirectionUnknown entries in
// existing using the extended outgoing cache. Entries that are resolved are
// replaced in-place; the slice order is preserved.
func (c *Client) ReEnrichUnknown(
	ctx context.Context,
	existing []*PaymentInfo,
	newCache []*lnrpc.Payment,
) []*PaymentInfo {
	outIdx := make(map[string]*lnrpc.Payment, len(newCache))
	for _, p := range newCache {
		outIdx[p.PaymentHash] = p
	}

	result := make([]*PaymentInfo, len(existing))
	for i, pi := range existing {
		if pi.Direction != DirectionUnknown {
			result[i] = pi
			continue
		}
		// Wrap as an AccountPayment and re-enrich.
		ap := &litrpc.AccountPayment{
			Hash:       pi.Hash,
			State:      pi.Status,
			FullAmount: pi.AmountSat,
		}
		result[i] = enrichSingle(ctx, c, ap, outIdx)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].TimestampNs > result[j].TimestampNs
	})
	return result
}

// OneWeekAgo returns the unix timestamp for 7 days ago.
func OneWeekAgo() uint64 {
	return uint64(time.Now().Add(-7 * 24 * time.Hour).Unix())
}

// enrichSingle resolves a single account payment.
func enrichSingle(
	ctx context.Context,
	c *Client,
	ap *litrpc.AccountPayment,
	outIdx map[string]*lnrpc.Payment,
) *PaymentInfo {
	hashHex := hex.EncodeToString(ap.Hash)

	// Attempt invoice lookup (incoming).
	inv, err := c.LookupInvoice(ctx, ap.Hash)
	if err == nil && inv != nil {
		amt := inv.AmtPaidSat
		if amt == 0 {
			amt = inv.Value
		}
		ts := inv.SettleDate * 1_000_000_000
		if ts == 0 {
			ts = inv.CreationDate * 1_000_000_000
		}
		return &PaymentInfo{
			Direction:   DirectionIncoming,
			Hash:        ap.Hash,
			HashHex:     hashHex,
			AmountSat:   amt,
			TimestampNs: ts,
			Memo:        inv.Memo,
			Status:      inv.State.String(),
			Invoice:     inv,
		}
	}

	// Search outgoing cache.
	if p, ok := outIdx[hashHex]; ok {
		return &PaymentInfo{
			Direction:   DirectionOutgoing,
			Hash:        ap.Hash,
			HashHex:     hashHex,
			AmountSat:   p.ValueSat,
			FeeSat:      p.FeeSat,
			TimestampNs: p.CreationTimeNs,
			Status:      p.Status.String(),
			Payment:     p,
		}
	}

	// Unknown — older than cache window or still in-flight.
	return &PaymentInfo{
		Direction: DirectionUnknown,
		Hash:      ap.Hash,
		HashHex:   hashHex,
		AmountSat: ap.FullAmount,
		Status:    ap.State,
	}
}
