package service

import (
	"crypto/ed25519"
	"github.com/gagliardetto/solana-go"
	"github.com/google/uuid"
	"github.com/stellar/go/exp/crypto/derivation"
	"github.com/tyler-smith/go-bip39"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
)

func DefaultWalletTokens(userID uuid.UUID) []model.WalletToken {
	return []model.WalletToken{
		{UserID: userID, Mint: solana.SolMint.String(), Name: "Solana", Symbol: "SOL", IsVisible: true},
		{UserID: userID, Mint: USDCMintAddress.String(), Name: "USDC", Symbol: "USDC", IsVisible: true},
		{UserID: userID, Mint: model.DuckPointMockMint, Name: model.DuckPointMockMint, Symbol: model.DuckPointMockMint, IsVisible: true},
	}
}

func generateMnemonic() (string, error) {
	entropy, err := bip39.NewEntropy(128)
	if err != nil {
		return "", apperrors.Internal("failed to generate an entropy", err)
	}

	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return "", apperrors.Internal("failed to generate a mnemonic phrase", err)
	}

	return mnemonic, nil
}

func generatePrivateKeyFromMnemonic(mnemonic string) (solana.PrivateKey, error) {
	seed := bip39.NewSeed(mnemonic, "")

	privateKey, err := deriveAccount(seed)
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}

const (
	SolanaDerivationPath = "m/44'/501'/0'/0'"
)

func deriveAccount(seed []byte) (solana.PrivateKey, error) {
	derivedPrivateKey, err := deriveEd25519Key(seed, SolanaDerivationPath)
	if err != nil {
		return nil, err
	}

	return solana.PrivateKey(derivedPrivateKey), nil
}

func deriveEd25519Key(seed []byte, derivationPath string) (ed25519.PrivateKey, error) {
	masterKey, err := derivation.DeriveForPath(derivationPath, seed)
	if err != nil {
		return nil, apperrors.Internal("failed to derive a seed", err)
	}

	return ed25519.NewKeyFromSeed(masterKey.Key), nil
}
