package client

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/lightninglabs/lightning-terminal/litrpc"
	"github.com/lightningnetwork/lnd/lnrpc"
)

// macaroonCredentials implements credentials.PerRPCCredentials.
// The stored hex value can be swapped at runtime via update(), which lets us
// upgrade from the bootstrap lit.macaroon to the freshly-baked supermacaroon
// without tearing down the gRPC connection.
type macaroonCredentials struct {
	mu  sync.RWMutex
	hex string
}

func (m *macaroonCredentials) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return map[string]string{"macaroon": m.hex}, nil
}

func (m *macaroonCredentials) RequireTransportSecurity() bool { return true }

func (m *macaroonCredentials) update(newHex string) {
	m.mu.Lock()
	m.hex = newHex
	m.mu.Unlock()
}

// Config holds the connection configuration.
type Config struct {
	RPCServer    string
	TLSCertPath  string
	MacaroonPath string
	Network      string
	LitDir       string
}

// DefaultConfig returns the default litd connection config.
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	litDir := filepath.Join(homeDir, ".lit")
	return &Config{
		RPCServer:    "localhost:8443",
		TLSCertPath:  filepath.Join(litDir, "tls.cert"),
		MacaroonPath: filepath.Join(litDir, "mainnet", "lit.macaroon"),
		Network:      "mainnet",
		LitDir:       litDir,
	}
}

// Client wraps all gRPC sub-clients for litd.
type Client struct {
	conn      *grpc.ClientConn
	Lightning lnrpc.LightningClient
	Accounts  litrpc.AccountsClient
	Sessions  litrpc.SessionsClient
	Proxy     litrpc.ProxyClient
}

// New dials litd, immediately bakes a supermacaroon, and returns a Client
// whose subsequent calls all use that supermacaroon.  The supermacaroon is
// held only in memory and discarded when the process exits.
func New(cfg *Config) (*Client, error) {
	tlsCreds, err := credentials.NewClientTLSFromFile(cfg.TLSCertPath, "")
	if err != nil {
		return nil, fmt.Errorf("loading TLS cert from %s: %w", cfg.TLSCertPath, err)
	}

	macBytes, err := os.ReadFile(cfg.MacaroonPath)
	if err != nil {
		return nil, fmt.Errorf("reading macaroon from %s: %w", cfg.MacaroonPath, err)
	}

	creds := &macaroonCredentials{hex: hex.EncodeToString(macBytes)}

	conn, err := grpc.NewClient(
		cfg.RPCServer,
		grpc.WithTransportCredentials(tlsCreds),
		grpc.WithPerRPCCredentials(creds),
	)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", cfg.RPCServer, err)
	}

	// Bake a supermacaroon so that a single credential covers all litd
	// sub-services (lnd, accounts, sessions, …).  This replaces the
	// bootstrap lit.macaroon in the shared credential holder; the file on
	// disk is never modified.
	proxy := litrpc.NewProxyClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	superResp, err := proxy.BakeSuperMacaroon(ctx, &litrpc.BakeSuperMacaroonRequest{})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("baking supermacaroon: %w", err)
	}

	creds.update(superResp.Macaroon)

	return &Client{
		conn:      conn,
		Proxy:     proxy,
		Lightning: lnrpc.NewLightningClient(conn),
		Accounts:  litrpc.NewAccountsClient(conn),
		Sessions:  litrpc.NewSessionsClient(conn),
	}, nil
}

// Close closes the underlying gRPC connection.
func (c *Client) Close() error {
	return c.conn.Close()
}
