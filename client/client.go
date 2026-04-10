package client

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	"github.com/lightninglabs/lightning-terminal/litrpc"
	"github.com/lightningnetwork/lnd/lnrpc"
)

// macaroonCredentials implements credentials.PerRPCCredentials for macaroon auth.
type macaroonCredentials struct {
	macaroon string
}

func (m *macaroonCredentials) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return map[string]string{
		"macaroon": m.macaroon,
	}, nil
}

func (m *macaroonCredentials) RequireTransportSecurity() bool {
	return true
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
	conn     *grpc.ClientConn
	Lightning lnrpc.LightningClient
	Accounts  litrpc.AccountsClient
	Sessions  litrpc.SessionsClient
}

// New creates a new Client connected to litd.
func New(cfg *Config) (*Client, error) {
	// Load TLS credentials.
	tlsCreds, err := credentials.NewClientTLSFromFile(cfg.TLSCertPath, "")
	if err != nil {
		return nil, fmt.Errorf("loading TLS cert from %s: %w", cfg.TLSCertPath, err)
	}

	// Load macaroon.
	macBytes, err := os.ReadFile(cfg.MacaroonPath)
	if err != nil {
		return nil, fmt.Errorf("reading macaroon from %s: %w", cfg.MacaroonPath, err)
	}
	macHex := hex.EncodeToString(macBytes)

	conn, err := grpc.NewClient(
		cfg.RPCServer,
		grpc.WithTransportCredentials(tlsCreds),
		grpc.WithPerRPCCredentials(&macaroonCredentials{macaroon: macHex}),
	)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", cfg.RPCServer, err)
	}

	return &Client{
		conn:      conn,
		Lightning: lnrpc.NewLightningClient(conn),
		Accounts:  litrpc.NewAccountsClient(conn),
		Sessions:  litrpc.NewSessionsClient(conn),
	}, nil
}

// Close closes the underlying gRPC connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// ContextWithMacaroon is a helper used internally — actual auth is via PerRPCCredentials.
func ContextWithMacaroon(ctx context.Context, macHex string) context.Context {
	md := metadata.Pairs("macaroon", macHex)
	return metadata.NewOutgoingContext(ctx, md)
}
