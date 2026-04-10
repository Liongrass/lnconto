package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lnconto/lnconto/client"
	"github.com/lnconto/lnconto/ui"
)

// version is set at build time.
var version = "dev"

func main() {
	cfg := client.DefaultConfig()

	// Parse global flags (same as litcli).
	args := os.Args[1:]
	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "--rpcserver" || arg == "-rpcserver":
			i++
			if i < len(args) {
				cfg.RPCServer = args[i]
			}
		case len(arg) > 13 && arg[:13] == "--rpcserver=":
			cfg.RPCServer = arg[13:]
		case arg == "--tlscertpath" || arg == "-tlscertpath":
			i++
			if i < len(args) {
				cfg.TLSCertPath = expandHome(args[i])
			}
		case len(arg) > 14 && arg[:14] == "--tlscertpath=":
			cfg.TLSCertPath = expandHome(arg[14:])
		case arg == "--macaroonpath" || arg == "-macaroonpath":
			i++
			if i < len(args) {
				cfg.MacaroonPath = expandHome(args[i])
			}
		case len(arg) > 15 && arg[:15] == "--macaroonpath=":
			cfg.MacaroonPath = expandHome(arg[15:])
		case arg == "--network" || arg == "-network":
			i++
			if i < len(args) {
				cfg.Network = args[i]
				cfg.MacaroonPath = defaultMacaroonPath(cfg.LitDir, cfg.Network)
			}
		case len(arg) > 10 && arg[:10] == "--network=":
			cfg.Network = arg[10:]
			cfg.MacaroonPath = defaultMacaroonPath(cfg.LitDir, cfg.Network)
		case arg == "--lit-dir" || arg == "-lit-dir" || arg == "--litdir" || arg == "-litdir":
			i++
			if i < len(args) {
				cfg.LitDir = expandHome(args[i])
				cfg.TLSCertPath = filepath.Join(cfg.LitDir, "tls.cert")
				cfg.MacaroonPath = defaultMacaroonPath(cfg.LitDir, cfg.Network)
			}
		case arg == "--version" || arg == "-version" || arg == "-v":
			fmt.Printf("lnconto %s\n", version)
			os.Exit(0)
		case arg == "--help" || arg == "-help" || arg == "-h":
			printHelp()
			os.Exit(0)
		}
		i++
	}

	c, err := client.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		fmt.Fprintf(os.Stderr, "Connection config:\n")
		fmt.Fprintf(os.Stderr, "  RPC server:    %s\n", cfg.RPCServer)
		fmt.Fprintf(os.Stderr, "  TLS cert:      %s\n", cfg.TLSCertPath)
		fmt.Fprintf(os.Stderr, "  Macaroon:      %s\n", cfg.MacaroonPath)
		fmt.Fprintf(os.Stderr, "\nUse --help for usage information.\n")
		os.Exit(1)
	}
	defer c.Close()

	model := ui.New(c)
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running lnconto: %v\n", err)
		os.Exit(1)
	}
}

func expandHome(path string) string {
	if len(path) > 1 && path[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func defaultMacaroonPath(litDir, network string) string {
	return filepath.Join(litDir, network, "lit.macaroon")
}

func printHelp() {
	fmt.Print(`lnconto - Interactive CLI for Lightning Terminal (litd)

USAGE:
  lnconto [global options]

GLOBAL OPTIONS:
  --rpcserver <host:port>   litd gRPC server address (default: localhost:8443)
  --tlscertpath <path>      TLS certificate path (default: ~/.lit/tls.cert)
  --macaroonpath <path>     Macaroon path (default: ~/.lit/mainnet/lit.macaroon)
  --network <network>       Network: mainnet, testnet, testnet4, regtest, simnet
                            (default: mainnet) — sets default macaroon path
  --lit-dir <path>          LiT data directory (default: ~/.lit)
  --version                 Show version
  --help                    Show this help

NAVIGATION:
  ↑/↓ or j/k    Navigate lists
  enter/space    Select item
  esc/q          Go back / quit

ACCOUNT ACTIONS (from account detail view):
  c              Credit account
  d              Debit account
  e              Set expiry date
  s              Create new LNC session for account
  m              Generate macaroon (account / readonly / invoice)

  S              Show sessions list (from dashboard)
  r              Refresh data
`)
}
