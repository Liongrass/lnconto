package client

import (
	"context"
	"fmt"

	"github.com/lightningnetwork/lnd/lnrpc"
)

// NodeInfo holds a summary of the node state.
type NodeInfo struct {
	Alias               string
	Pubkey              string
	NumActiveChannels   uint32
	NumPendingChannels  uint32
	NumInactiveChannels uint32
	OnchainBalance      int64 // confirmed sats
	OffchainBalance     int64 // local balance across channels
	BlockHeight         uint32
	SyncedToChain       bool
}

// GetNodeInfo fetches node info, wallet balance, and channel balance in parallel.
func (c *Client) GetNodeInfo(ctx context.Context) (*NodeInfo, error) {
	type result[T any] struct {
		val T
		err error
	}

	infoCh := make(chan result[*lnrpc.GetInfoResponse], 1)
	walletCh := make(chan result[*lnrpc.WalletBalanceResponse], 1)
	chanCh := make(chan result[*lnrpc.ChannelBalanceResponse], 1)

	go func() {
		r, err := c.Lightning.GetInfo(ctx, &lnrpc.GetInfoRequest{})
		infoCh <- result[*lnrpc.GetInfoResponse]{r, err}
	}()
	go func() {
		r, err := c.Lightning.WalletBalance(ctx, &lnrpc.WalletBalanceRequest{})
		walletCh <- result[*lnrpc.WalletBalanceResponse]{r, err}
	}()
	go func() {
		r, err := c.Lightning.ChannelBalance(ctx, &lnrpc.ChannelBalanceRequest{})
		chanCh <- result[*lnrpc.ChannelBalanceResponse]{r, err}
	}()

	infoRes := <-infoCh
	if infoRes.err != nil {
		return nil, fmt.Errorf("GetInfo: %w", infoRes.err)
	}
	walletRes := <-walletCh
	if walletRes.err != nil {
		return nil, fmt.Errorf("WalletBalance: %w", walletRes.err)
	}
	chanRes := <-chanCh
	if chanRes.err != nil {
		return nil, fmt.Errorf("ChannelBalance: %w", chanRes.err)
	}

	var offchain int64
	if chanRes.val.LocalBalance != nil {
		offchain = int64(chanRes.val.LocalBalance.Sat)
	}

	return &NodeInfo{
		Alias:               infoRes.val.Alias,
		Pubkey:              infoRes.val.IdentityPubkey,
		NumActiveChannels:   infoRes.val.NumActiveChannels,
		NumPendingChannels:  infoRes.val.NumPendingChannels,
		NumInactiveChannels: infoRes.val.NumInactiveChannels,
		OnchainBalance:      walletRes.val.ConfirmedBalance,
		OffchainBalance:     offchain,
		BlockHeight:         infoRes.val.BlockHeight,
		SyncedToChain:       infoRes.val.SyncedToChain,
	}, nil
}
