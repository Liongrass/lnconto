# lnconto

Interactive terminal UI for [Lightning Terminal](https://github.com/lightninglabs/lightning-terminal) (`litd`). Manage accounts, sessions, and macaroons without leaving your terminal.

![Go](https://img.shields.io/badge/go-1.21+-blue)

## Requirements

- Go 1.21+
- A running `litd` instance

## Install

```sh
make install
```

Or build locally:

```sh
make
./lnconto
```

## Usage

```
lnconto [options]

  --rpcserver <host:port>   litd gRPC address       (default: localhost:8443)
  --tlscertpath <path>      TLS certificate          (default: ~/.lit/tls.cert)
  --macaroonpath <path>     Macaroon file            (default: ~/.lit/mainnet/lit.macaroon)
  --network <network>       mainnet / testnet / regtest / simnet
  --lit-dir <path>          LiT data directory       (default: ~/.lit)
```

## Keys

| Key | Action |
|-----|--------|
| `↑ / ↓` or `j / k` | Navigate |
| `Enter` | Select account |
| `S` | Sessions list |
| `r` | Refresh |
| `Esc / q` | Back / quit |

**In account detail:**

| Key | Action |
|-----|--------|
| `c` | Credit account |
| `d` | Debit account |
| `e` | Set expiry date |
| `s` | New LNC session |
| `m` | Generate macaroon (full / readonly / invoice) |
