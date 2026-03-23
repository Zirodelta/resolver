package resolver

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	confirm "github.com/gagliardetto/solana-go/rpc/ws"
	"go.uber.org/zap"

	"github.com/Zirodelta/resolver/pkg/pda"
	"github.com/Zirodelta/resolver/pkg/state"
	"github.com/Zirodelta/resolver/pkg/types"
)

// Resolver submits resolve_market_permissionless transactions.
type Resolver struct {
	client    *rpc.Client
	wsURL     string
	programID solana.PublicKey
	wallet    solana.PrivateKey
	dryRun    bool
	logger    *zap.Logger
}

// New creates a resolver.
func New(client *rpc.Client, wsURL string, programID solana.PublicKey, wallet solana.PrivateKey, dryRun bool, logger *zap.Logger) *Resolver {
	return &Resolver{
		client:    client,
		wsURL:     wsURL,
		programID: programID,
		wallet:    wallet,
		dryRun:    dryRun,
		logger:    logger,
	}
}

// instructionDiscriminator returns SHA256("global:resolve_market_permissionless")[0:8].
func instructionDiscriminator() [8]byte {
	h := sha256.Sum256([]byte("global:resolve_market_permissionless"))
	var disc [8]byte
	copy(disc[:], h[:8])
	return disc
}

// Resolve submits a resolve_market_permissionless TX for the given market.
func (r *Resolver) Resolve(ctx context.Context, market *state.MarketState) error {
	log := r.logger.With(
		zap.String("market_id", market.MarketIDHex()),
		zap.String("symbol", market.Symbol),
		zap.String("exchange", market.Exchange),
	)

	if r.dryRun {
		log.Info("DRY RUN: would resolve market")
		return nil
	}

	// Derive PDAs.
	vaultState, _, err := pda.FindVaultStatePDA(r.programID)
	if err != nil {
		return fmt.Errorf("derive vault_state PDA: %w", err)
	}

	marketState, _, err := pda.FindMarketStatePDA(r.programID, market.MarketID)
	if err != nil {
		return fmt.Errorf("derive market_state PDA: %w", err)
	}

	resolverPubkey := r.wallet.PublicKey()

	// Vault USDC ATA (Token-2022).
	vaultUSDCATA, _, err := pda.FindATA(vaultState, types.USDCMint, types.TokenProgram)
	if err != nil {
		return fmt.Errorf("derive vault USDC ATA: %w", err)
	}

	// Resolver USDC ATA (standard SPL Token for resolver's own ATA).
	resolverUSDCATA, _, err := pda.FindATA(resolverPubkey, types.USDCMint, types.TokenProgram)
	if err != nil {
		return fmt.Errorf("derive resolver USDC ATA: %w", err)
	}

	// Build instruction data: discriminator (8) + market_id (16) = 24 bytes.
	disc := instructionDiscriminator()
	ixData := make([]byte, 24)
	copy(ixData[0:8], disc[:])
	copy(ixData[8:24], market.MarketID[:])

	// Build the instruction.
	ix := &solana.GenericInstruction{
		ProgID: r.programID,
		AccountValues: solana.AccountMetaSlice{
			{PublicKey: vaultState, IsWritable: false, IsSigner: false},       // vault_state (read-only)
			{PublicKey: marketState, IsWritable: true, IsSigner: false},       // market_state (mut)
			{PublicKey: market.FeedHash, IsWritable: false, IsSigner: false},  // feed (Switchboard, read-only)
			{PublicKey: resolverPubkey, IsWritable: true, IsSigner: true},     // resolver (signer, mut)
			{PublicKey: vaultUSDCATA, IsWritable: true, IsSigner: false},      // vault_usdc_ata (mut)
			{PublicKey: resolverUSDCATA, IsWritable: true, IsSigner: false},   // resolver_usdc_ata (mut)
			{PublicKey: types.USDCMint, IsWritable: false, IsSigner: false},   // usdc_mint
			{PublicKey: types.TokenProgram, IsWritable: false, IsSigner: false}, // token_program
		},
		DataBytes: ixData,
	}

	// Get recent blockhash.
	recent, err := r.client.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return fmt.Errorf("get blockhash: %w", err)
	}

	// Build and sign transaction.
	tx, err := solana.NewTransaction(
		[]solana.Instruction{ix},
		recent.Value.Blockhash,
		solana.TransactionPayer(resolverPubkey),
	)
	if err != nil {
		return fmt.Errorf("build transaction: %w", err)
	}

	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(resolverPubkey) {
			return &r.wallet
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("sign transaction: %w", err)
	}

	// Submit.
	sig, err := r.client.SendTransactionWithOpts(ctx, tx, rpc.TransactionOpts{
		SkipPreflight: false,
	})
	if err != nil {
		return fmt.Errorf("send transaction: %w", err)
	}

	log.Info("transaction submitted", zap.String("signature", sig.String()))

	// Confirm via WebSocket if available, otherwise poll.
	if err := r.confirmTransaction(ctx, sig); err != nil {
		log.Warn("confirmation failed (TX may still land)", zap.Error(err))
	} else {
		log.Info("transaction confirmed", zap.String("signature", sig.String()))
	}

	return nil
}

func (r *Resolver) confirmTransaction(ctx context.Context, sig solana.Signature) error {
	// Try WebSocket confirmation.
	wsClient, err := confirm.Connect(ctx, r.wsURL)
	if err != nil {
		// Fall back to polling.
		return r.pollConfirmation(ctx, sig)
	}
	defer wsClient.Close()

	sub, err := wsClient.SignatureSubscribe(sig, rpc.CommitmentFinalized)
	if err != nil {
		return r.pollConfirmation(ctx, sig)
	}
	defer sub.Unsubscribe()

	timer := time.NewTimer(60 * time.Second)
	defer timer.Stop()

	select {
	case result, ok := <-sub.Response():
		if !ok {
			return fmt.Errorf("subscription closed")
		}
		if result.Value.Err != nil {
			return fmt.Errorf("transaction failed: %v", result.Value.Err)
		}
		return nil
	case <-timer.C:
		return fmt.Errorf("confirmation timeout (60s)")
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Resolver) pollConfirmation(ctx context.Context, sig solana.Signature) error {
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := r.client.GetSignatureStatuses(ctx, false, sig)
		if err == nil && resp != nil && resp.Value != nil && len(resp.Value) > 0 && resp.Value[0] != nil {
			if resp.Value[0].Err != nil {
				return fmt.Errorf("transaction failed: %v", resp.Value[0].Err)
			}
			if resp.Value[0].ConfirmationStatus == rpc.ConfirmationStatusFinalized ||
				resp.Value[0].ConfirmationStatus == rpc.ConfirmationStatusConfirmed {
				return nil
			}
		}
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return fmt.Errorf("confirmation timeout (60s)")
}
