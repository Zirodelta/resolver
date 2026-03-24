// Package metrics provides Prometheus instrumentation for the resolver daemon.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// CyclesTotal counts scan cycles.
	CyclesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "resolver_cycles_total",
		Help: "Total scan cycles executed.",
	})

	// MarketsScanned counts markets found per cycle.
	MarketsScanned = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "resolver_markets_scanned",
		Help: "Number of resolvable markets found in the last scan cycle.",
	})

	// ResolutionsTotal counts resolution attempts by outcome.
	ResolutionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "resolver_resolutions_total",
		Help: "Total resolution attempts by status (success, failure, dry_run).",
	}, []string{"status"})

	// ResolutionDuration tracks time to resolve a single market.
	ResolutionDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "resolver_resolution_duration_seconds",
		Help:    "Time to resolve a single market (TX build + submit + confirm).",
		Buckets: []float64{1, 2, 5, 10, 15, 30, 60},
	})

	// ScanDuration tracks time per scan cycle.
	ScanDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "resolver_scan_duration_seconds",
		Help:    "Time to scan for resolvable markets.",
		Buckets: []float64{0.1, 0.5, 1, 2, 5, 10},
	})

	// LastCycleTimestamp is the unix timestamp of the last completed cycle.
	LastCycleTimestamp = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "resolver_last_cycle_timestamp_seconds",
		Help: "Unix timestamp of the last completed scan cycle.",
	})

	// RPCRequestsTotal counts Solana RPC calls.
	RPCRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "resolver_rpc_requests_total",
		Help: "Total Solana RPC requests by method and status.",
	}, []string{"method", "status"})

	// WalletSOLBalance tracks the resolver wallet SOL balance.
	WalletSOLBalance = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "resolver_wallet_sol_balance",
		Help: "SOL balance of the resolver wallet (in SOL).",
	})

	// TipEarnedTotal tracks cumulative USDC tips earned from resolutions.
	TipEarnedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "resolver_tip_earned_usdc_total",
		Help: "Cumulative USDC tips earned from permissionless resolution.",
	})
)
