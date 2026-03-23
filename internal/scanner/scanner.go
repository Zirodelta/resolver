package scanner

import (
	"context"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"go.uber.org/zap"

	"github.com/Zirodelta/resolver/pkg/state"
	"github.com/Zirodelta/resolver/pkg/types"
)

// Scanner finds unresolved markets on-chain.
type Scanner struct {
	client    *rpc.Client
	programID solana.PublicKey
	logger    *zap.Logger
}

// New creates a scanner for the given program.
func New(client *rpc.Client, programID solana.PublicKey, logger *zap.Logger) *Scanner {
	return &Scanner{
		client:    client,
		programID: programID,
		logger:    logger,
	}
}

// FindClosedMarkets returns all markets with status=Closed that are past settlement time.
func (s *Scanner) FindClosedMarkets(ctx context.Context) ([]*state.MarketState, error) {
	resp, err := s.client.GetProgramAccountsWithOpts(ctx, s.programID, &rpc.GetProgramAccountsOpts{
		Encoding: solana.EncodingBase64,
		Filters: []rpc.RPCFilter{
			{
				Memcmp: &rpc.RPCFilterMemcmp{
					Offset: 0,
					Bytes:  solana.Base58(state.Discriminator[:]),
				},
			},
			{
				Memcmp: &rpc.RPCFilterMemcmp{
					Offset: 132,
					Bytes:  solana.Base58([]byte{byte(types.StatusClosed)}),
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	var markets []*state.MarketState
	for _, acct := range resp {
		data := acct.Account.Data.GetBinary()
		if len(data) == 0 {
			continue
		}

		market, err := state.DecodeMarketState(data)
		if err != nil {
			s.logger.Warn("failed to decode market account",
				zap.String("account", acct.Pubkey.String()),
				zap.Error(err),
			)
			continue
		}
		market.AccountKey = acct.Pubkey

		if !market.IsReadyToResolve() {
			s.logger.Debug("market closed but not yet past settlement time",
				zap.String("market_id", market.MarketIDHex()),
				zap.String("symbol", market.Symbol),
				zap.Time("settlement_ts", market.SettlementTS),
			)
			continue
		}

		s.logger.Info("found resolvable market",
			zap.String("market_id", market.MarketIDHex()),
			zap.String("symbol", market.Symbol),
			zap.String("exchange", market.Exchange),
			zap.String("account", acct.Pubkey.String()),
		)
		markets = append(markets, market)
	}

	return markets, nil
}
