package pda

import "github.com/gagliardetto/solana-go"

// FindVaultStatePDA derives the vault_state PDA: seeds=[b"vault_state"].
func FindVaultStatePDA(programID solana.PublicKey) (solana.PublicKey, uint8, error) {
	addr, bump, err := solana.FindProgramAddress(
		[][]byte{[]byte("vault_state")},
		programID,
	)
	return addr, bump, err
}

// FindMarketStatePDA derives the market_state PDA: seeds=[b"market", market_id].
func FindMarketStatePDA(programID solana.PublicKey, marketID [16]byte) (solana.PublicKey, uint8, error) {
	addr, bump, err := solana.FindProgramAddress(
		[][]byte{[]byte("market"), marketID[:]},
		programID,
	)
	return addr, bump, err
}

// FindATA derives the Associated Token Account address.
// For Token-2022 mints, tokenProgramID should be the Token-2022 program.
func FindATA(wallet, mint, tokenProgramID solana.PublicKey) (solana.PublicKey, uint8, error) {
	addr, bump, err := solana.FindProgramAddress(
		[][]byte{
			wallet.Bytes(),
			tokenProgramID.Bytes(),
			mint.Bytes(),
		},
		solana.MustPublicKeyFromBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL"),
	)
	return addr, bump, err
}
