package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/Zirodelta/resolver/internal/resolver"
	"github.com/Zirodelta/resolver/internal/scanner"
)

type config struct {
	RPCURL       string
	WSURL        string
	Wallet       solana.PrivateKey
	ProgramID    solana.PublicKey
	PollInterval time.Duration
	DryRun       bool
	LogLevel     zapcore.Level
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.LogLevel)
	defer logger.Sync()

	logger.Info("starting settled resolver",
		zap.String("program_id", cfg.ProgramID.String()),
		zap.String("resolver", cfg.Wallet.PublicKey().String()),
		zap.Duration("poll_interval", cfg.PollInterval),
		zap.Bool("dry_run", cfg.DryRun),
	)

	client := rpc.New(cfg.RPCURL)

	scan := scanner.New(client, cfg.ProgramID, logger)
	res := resolver.New(client, cfg.WSURL, cfg.ProgramID, cfg.Wallet, cfg.DryRun, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Graceful shutdown on SIGINT/SIGTERM.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logger.Info("received signal, shutting down", zap.String("signal", sig.String()))
		cancel()
	}()

	// Main loop.
	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	// Run immediately on start, then on tick.
	runCycle(ctx, scan, res, logger)

	for {
		select {
		case <-ticker.C:
			runCycle(ctx, scan, res, logger)
		case <-ctx.Done():
			logger.Info("resolver stopped")
			return
		}
	}
}

func runCycle(ctx context.Context, scan *scanner.Scanner, res *resolver.Resolver, logger *zap.Logger) {
	markets, err := scan.FindClosedMarkets(ctx)
	if err != nil {
		logger.Error("scan failed", zap.Error(err))
		return
	}

	if len(markets) == 0 {
		logger.Debug("no resolvable markets found")
		return
	}

	logger.Info("found resolvable markets", zap.Int("count", len(markets)))

	for _, market := range markets {
		if ctx.Err() != nil {
			return
		}
		if err := res.Resolve(ctx, market); err != nil {
			logger.Error("resolve failed",
				zap.String("market_id", market.MarketIDHex()),
				zap.String("symbol", market.Symbol),
				zap.Error(err),
			)
			// Continue to next market — don't let one failure block others.
		}
	}
}

func loadConfig() (*config, error) {
	rpcURL := os.Getenv("SOLANA_RPC_URL")
	if rpcURL == "" {
		return nil, fmt.Errorf("SOLANA_RPC_URL is required")
	}

	// Derive WebSocket URL from RPC URL.
	wsURL := rpcToWS(rpcURL)

	keypairRaw := os.Getenv("RESOLVER_KEYPAIR")
	if keypairRaw == "" {
		return nil, fmt.Errorf("RESOLVER_KEYPAIR is required")
	}

	wallet, err := loadKeypair(keypairRaw)
	if err != nil {
		return nil, fmt.Errorf("load keypair: %w", err)
	}

	programIDStr := getEnvDefault("PROGRAM_ID", "7rLM8d27AgkbFjQfJJpHzD4A5pMttD7PzMrqDiMNf7AW")
	programID, err := solana.PublicKeyFromBase58(programIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid PROGRAM_ID: %w", err)
	}

	pollStr := getEnvDefault("POLL_INTERVAL", "30s")
	pollInterval, err := time.ParseDuration(pollStr)
	if err != nil {
		return nil, fmt.Errorf("invalid POLL_INTERVAL: %w", err)
	}

	dryRun := getEnvDefault("DRY_RUN", "false") == "true"

	logLevel := zapcore.InfoLevel
	if lvl := os.Getenv("LOG_LEVEL"); lvl != "" {
		if err := logLevel.UnmarshalText([]byte(lvl)); err != nil {
			return nil, fmt.Errorf("invalid LOG_LEVEL: %w", err)
		}
	}

	return &config{
		RPCURL:       rpcURL,
		WSURL:        wsURL,
		Wallet:       wallet,
		ProgramID:    programID,
		PollInterval: pollInterval,
		DryRun:       dryRun,
		LogLevel:     logLevel,
	}, nil
}

// loadKeypair loads a Solana keypair from either a file path (JSON array) or base58 string.
func loadKeypair(raw string) (solana.PrivateKey, error) {
	// If it looks like a file path, read it.
	if strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "~") {
		data, err := os.ReadFile(raw)
		if err != nil {
			return nil, fmt.Errorf("read keypair file %s: %w", raw, err)
		}
		var keyBytes []byte
		if err := json.Unmarshal(data, &keyBytes); err != nil {
			return nil, fmt.Errorf("parse keypair JSON: %w", err)
		}
		return solana.PrivateKey(keyBytes), nil
	}

	// Otherwise treat as base58.
	wallet, err := solana.PrivateKeyFromBase58(raw)
	if err != nil {
		return nil, fmt.Errorf("parse base58 keypair: %w", err)
	}
	return wallet, nil
}

func rpcToWS(rpcURL string) string {
	ws := strings.Replace(rpcURL, "https://", "wss://", 1)
	ws = strings.Replace(ws, "http://", "ws://", 1)
	return ws
}

func getEnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func newLogger(level zapcore.Level) *zap.Logger {
	cfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(level),
		Encoding:         "console",
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	logger, err := cfg.Build()
	if err != nil {
		// Fallback to nop if config fails.
		return zap.NewNop()
	}
	return logger
}
