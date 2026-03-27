package state

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/gagliardetto/solana-go"

	"github.com/Zirodelta/resolver/pkg/types"
)

// MarketState represents a deserialized on-chain market account.
type MarketState struct {
	MarketID      [16]byte
	Symbol        string
	Exchange      string
	SettlementTS  time.Time
	OpenTS        time.Time
	CloseTS       time.Time
	QYes          int64
	QNo           int64
	BParameter    uint64
	TotalVolume   uint64
	TradeCount    uint32
	Status        types.MarketStatus
	HasOutcome    bool
	Outcome       types.Outcome
	HasActualRate bool
	ActualRate    int64
	Authority     solana.PublicKey
	Bump          uint8
	YesMint       solana.PublicKey
	NoMint        solana.PublicKey
	FeedHash      solana.PublicKey // Switchboard feed pubkey stored as 32 bytes

	// AccountKey is the on-chain address of this account (set by scanner).
	AccountKey solana.PublicKey
}

// Discriminator for the MarketState account: SHA256("account:MarketState")[0:8].
var Discriminator [8]byte

func init() {
	h := sha256.Sum256([]byte("account:MarketState"))
	copy(Discriminator[:], h[:8])
}

// Minimum sizes for backward-compatible decoding.
// v1: original struct without NoMint/FeedHash (209 bytes)
// v2: added NoMint (241 bytes)
// v3: added FeedHash (273 bytes, current)
const (
	marketAccountSizeV1 = 209 // up to and including YesMint
	marketAccountSizeV2 = 241 // added NoMint
	marketAccountSizeV3 = 273 // added FeedHash (current)
)

// DecodeMarketState deserializes raw account bytes into a MarketState.
// Backward-compatible: handles v1 (209 bytes) accounts that predate NoMint/FeedHash fields.
func DecodeMarketState(data []byte) (*MarketState, error) {
	if len(data) < marketAccountSizeV1 {
		return nil, fmt.Errorf("account data too short: %d bytes (need >= %d)", len(data), marketAccountSizeV1)
	}

	m := &MarketState{}

	copy(m.MarketID[:], data[8:24])
	m.Symbol = strings.TrimRight(string(data[24:56]), "\x00")
	m.Exchange = strings.TrimRight(string(data[56:72]), "\x00")
	m.SettlementTS = time.Unix(int64(binary.LittleEndian.Uint64(data[72:80])), 0)
	m.OpenTS = time.Unix(int64(binary.LittleEndian.Uint64(data[80:88])), 0)
	m.CloseTS = time.Unix(int64(binary.LittleEndian.Uint64(data[88:96])), 0)
	m.QYes = int64(binary.LittleEndian.Uint64(data[96:104]))
	m.QNo = int64(binary.LittleEndian.Uint64(data[104:112]))
	m.BParameter = binary.LittleEndian.Uint64(data[112:120])
	m.TotalVolume = binary.LittleEndian.Uint64(data[120:128])
	m.TradeCount = binary.LittleEndian.Uint32(data[128:132])
	m.Status = types.MarketStatus(data[132])

	// offset 133: outcome Option<Outcome> (2 bytes: 0=None, 1+variant)
	if data[133] == 1 {
		m.HasOutcome = true
		m.Outcome = types.Outcome(data[134])
	}

	// offset 135: actual_rate Option<i64> (9 bytes: 0=None, 1+i64)
	if data[135] == 1 {
		m.HasActualRate = true
		m.ActualRate = int64(binary.LittleEndian.Uint64(data[136:144]))
	}

	m.Authority = solana.PublicKeyFromBytes(data[144:176])
	m.Bump = data[176]
	m.YesMint = solana.PublicKeyFromBytes(data[177:209])

	// v2+ fields: NoMint (added after initial deployment)
	if len(data) >= marketAccountSizeV2 {
		m.NoMint = solana.PublicKeyFromBytes(data[209:241])
	}

	// v3+ fields: FeedHash (Switchboard price feed)
	if len(data) >= marketAccountSizeV3 {
		m.FeedHash = solana.PublicKeyFromBytes(data[241:273])
	}

	return m, nil
}

// IsReadyToResolve returns true if the market is Closed and past settlement time.
func (m *MarketState) IsReadyToResolve() bool {
	return m.Status == types.StatusClosed && time.Now().After(m.SettlementTS)
}

// MarketIDHex returns the market_id as a hex string for logging.
func (m *MarketState) MarketIDHex() string {
	return fmt.Sprintf("%x", m.MarketID)
}
