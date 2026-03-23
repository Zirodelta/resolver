package types

import "github.com/gagliardetto/solana-go"

// Settled program ID (devnet default).
const DefaultProgramID = "7rLM8d27AgkbFjQfJJpHzD4A5pMttD7PzMrqDiMNf7AW"

// Token mints and programs.
var (
	USDCMint        = solana.MustPublicKeyFromBase58("yaueFp1jYnuciyHeMHG39GCckqvAKBBB1aZavqQ5HFL")
	TokenProgram    = solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	Token2022       = solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")
	ATAProgram      = solana.MustPublicKeyFromBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL")
	SystemProgram   = solana.SystemProgramID
)

// MarketStatus represents on-chain market state.
type MarketStatus uint8

const (
	StatusPending  MarketStatus = 0
	StatusOpen     MarketStatus = 1
	StatusClosed   MarketStatus = 2
	StatusResolved MarketStatus = 3
	StatusSettled  MarketStatus = 4
)

func (s MarketStatus) String() string {
	switch s {
	case StatusPending:
		return "Pending"
	case StatusOpen:
		return "Open"
	case StatusClosed:
		return "Closed"
	case StatusResolved:
		return "Resolved"
	case StatusSettled:
		return "Settled"
	default:
		return "Unknown"
	}
}

// Outcome from market resolution.
type Outcome uint8

const (
	OutcomeYes  Outcome = 0
	OutcomeNo   Outcome = 1
	OutcomeVoid Outcome = 2
)

// MarketAccount size including discriminator.
const MarketAccountSize = 305
