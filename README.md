# Settled Resolver

Permissionless resolver daemon for the [Settled](https://settled.market) prediction market protocol on Solana.

Scans for closed markets past their settlement time, calls `resolve_market_permissionless` on each, and earns a 10 bps USDC tip per resolution.

## Quick Start

### 1. Prerequisites

- Go 1.22+ (or Docker)
- A funded Solana keypair (needs SOL for gas + a USDC token account for tips)

### 2. Generate a keypair (if you don't have one)

```bash
solana-keygen new -o resolver-keypair.json
# Fund it with devnet SOL:
solana airdrop 2 $(solana-keygen pubkey resolver-keypair.json) --url devnet
```

### 3. Configure

```bash
cp .env.example .env
# Edit .env — set SOLANA_RPC_URL and RESOLVER_KEYPAIR at minimum
```

### 4. Run

**With Go:**
```bash
make run
```

**With Docker:**
```bash
docker compose up -d
```

### 5. Verify

Logs show scanned markets and submitted transactions:
```
INFO  starting settled resolver  {"program_id": "7rLM...", "resolver": "YourKey...", "poll_interval": "30s"}
INFO  found resolvable markets   {"count": 2}
INFO  transaction submitted      {"market_id": "abc123...", "signature": "5xK..."}
INFO  transaction confirmed      {"market_id": "abc123...", "signature": "5xK..."}
```

## Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SOLANA_RPC_URL` | Yes | — | Solana RPC endpoint |
| `RESOLVER_KEYPAIR` | Yes | — | Path to JSON keypair or base58 private key |
| `PROGRAM_ID` | No | `7rLM8d27...` | Settled program ID |
| `POLL_INTERVAL` | No | `30s` | How often to scan for markets |
| `LOG_LEVEL` | No | `info` | `debug`, `info`, `warn`, `error` |
| `DRY_RUN` | No | `false` | Log resolvable markets without submitting TXs |

## How It Works

1. **Scan** — `getProgramAccounts` with memcmp filters for MarketState discriminator + status=Closed
2. **Filter** — Only markets past their `settlement_ts` are eligible
3. **Resolve** — Builds and submits `resolve_market_permissionless` instruction for each market
4. **Confirm** — Waits for finalized confirmation via WebSocket (falls back to polling)
5. **Repeat** — Sleeps for `POLL_INTERVAL`, then scans again

Failed resolutions (already resolved, oracle unavailable, etc.) are logged and skipped — the daemon continues to the next market.

## Monitoring

The resolver exposes Prometheus metrics at `/metrics` (default `:8082`).

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `METRICS_ADDR` | No | `:8082` | Address for the metrics HTTP server |

### Available Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `resolver_cycles_total` | counter | Total scan cycles completed |
| `resolver_markets_scanned` | gauge | Markets found in last scan |
| `resolver_resolutions_total{status}` | counter | Resolutions by status (success/failure) |
| `resolver_scan_duration_seconds` | histogram | Scan cycle duration |
| `resolver_resolution_duration_seconds` | histogram | Per-market resolution duration |
| `resolver_last_cycle_timestamp_seconds` | gauge | Unix timestamp of last scan |

### Ready-Made Configs

The `monitoring/` directory includes:

- **`prometheus.yml`** — Sample scrape config to add to your Prometheus
- **`alerts.yml`** — Alert rules (heartbeat stale, resolution failures, slow scans)
- **`grafana-dashboard.json`** — Import into Grafana for instant visibility

```bash
# Add the scrape config to your prometheus.yml, then import the dashboard:
# Grafana → Dashboards → Import → Upload JSON → monitoring/grafana-dashboard.json
```

## Architecture

```
cmd/resolver/main.go        — entry point, config, signal handling, main loop
internal/scanner/            — getProgramAccounts with memcmp filters
internal/resolver/           — TX building, signing, submission, confirmation
pkg/pda/                     — PDA derivation (vault_state, market_state, ATAs)
pkg/state/                   — MarketState account deserialization
pkg/types/                   — constants, enums
```

## License

Apache-2.0
