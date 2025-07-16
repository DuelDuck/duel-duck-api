package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"gitlab.com/duel-duck/duel-duck-api/config"
	"gitlab.com/duel-duck/duel-duck-api/internal/client/jupiter"
	"gitlab.com/duel-duck/duel-duck-api/internal/client/solscan"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/internal/storage/cache"
	"gitlab.com/duel-duck/duel-duck-api/internal/storage/cypher"
	"gitlab.com/duel-duck/duel-duck-api/internal/storage/repository"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	repo "gitlab.com/duel-duck/duel-duck-api/pkg/repository"
	"gitlab.com/duel-duck/duel-duck-api/pkg/sigtracker"
)

type WalletService struct {
	TaskService           *TaskService
	PriorityTracker       *PriorityTracker
	SigTracker            *sigtracker.TxTracker
	UserRepository        *repository.UserRepository
	WalletTokenRepository *repository.WalletTokenRepository
	WalletCacheStorage    *cache.WalletCacheStorage
	TxNotificationStorage *cache.TxNotificationStorage
	privateKeyRepository  *cypher.PrivateKeyRepository
	SolanaRPC             *rpc.Client
	Jupiter               *jupiter.Client
	Solscan               *solscan.Client
	HTTPClient            *resty.Client
	TransactionManager    *repo.TransactionManager

	adminPrivateKey           solana.PrivateKey
	SwapCommissionCoefficient float64
	contractAddress           string
	contractAddressAPI        string
	moralisAPIKey             string
}

func NewWalletService(
	c *config.Config,
	taskService *TaskService,
	tracker *PriorityTracker,
	sigTracker *sigtracker.TxTracker,
	solanaRPC *rpc.Client,
	userRepository *repository.UserRepository,
	privateKeyRepository *cypher.PrivateKeyRepository,
	walletCacheStorage *cache.WalletCacheStorage,
	walletTokenRepository *repository.WalletTokenRepository,
	txNotificationStorage *cache.TxNotificationStorage,
	jupiter *jupiter.Client,
	solscan *solscan.Client,
	transactionManager *repo.TransactionManager,
) (*WalletService, error) {
	adminPrivateKey, err := solana.PrivateKeyFromBase58(c.App.SolanaAdminPrivateKey)
	if err != nil {
		return nil, apperrors.Internal("failed to get solana contract admin private key")
	}

	sigTracker.Start()

	return &WalletService{
		TaskService:               taskService,
		PriorityTracker:           tracker,
		SigTracker:                sigTracker,
		UserRepository:            userRepository,
		privateKeyRepository:      privateKeyRepository,
		adminPrivateKey:           adminPrivateKey,
		WalletCacheStorage:        walletCacheStorage,
		WalletTokenRepository:     walletTokenRepository,
		TxNotificationStorage:     txNotificationStorage,
		SolanaRPC:                 solanaRPC,
		HTTPClient:                resty.New(),
		Jupiter:                   jupiter,
		Solscan:                   solscan,
		SwapCommissionCoefficient: c.App.SwapCommissionCoefficient,
		contractAddress:           c.App.ContractAddress,
		contractAddressAPI:        c.App.ContractAddressAPI,
		moralisAPIKey:             c.Client.MoralisAPIKey,
		TransactionManager:        transactionManager,
	}, nil
}

const (
	pemBlockTypePublicKey = "PUBLIC KEY"
)

func (s *WalletService) encryptWallet(publicKeyPem string, data string) (string, error) {
	block, _ := pem.Decode([]byte(publicKeyPem))
	if block == nil || block.Type != pemBlockTypePublicKey {
		return "", apperrors.BadRequest("invalid format of public key")
	}

	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", apperrors.BadRequest("failed to parse public key", err)
	}

	rsaPubKey, ok := pubKey.(*rsa.PublicKey)
	if !ok {
		return "", apperrors.BadRequest("public key is not rsa")
	}

	encryptedBytes, err := rsa.EncryptOAEP(
		sha256.New(),
		rand.Reader,
		rsaPubKey,
		[]byte(data),
		nil)
	if err != nil {
		return "", apperrors.Internal("failed to encrypt wallet data", err)
	}

	return base64.StdEncoding.EncodeToString(encryptedBytes), nil
}

func (s *WalletService) ShowWallet(
	ctx context.Context,
	userID uuid.UUID,
	publicCypherKey string,
) (string, error) {
	wallet, err := s.privateKeyRepository.GetWalletData(ctx, userID)
	if err != nil {
		return "", err
	}

	walletData := wallet.Mnemonic
	if walletData == "" {
		walletData = wallet.PrivateKey
	}

	encryptedWallet, err := s.encryptWallet(publicCypherKey, walletData)
	if err != nil {
		return "", err
	}

	return encryptedWallet, nil
}

func (s *WalletService) GetTransactions(
	ctx context.Context,
	publicAddress string,
	req *model.GetTransactionHistoryReq,
) ([]model.Transaction, error) {
	return s.Solscan.GetTransactions(ctx, publicAddress, req)
}

func (s *WalletService) GetTokenAccounts(
	ctx context.Context,
	publicAddress string,
	req *model.GetTokenAccountsReq,
) (*model.SolscanTokenAccountsResp, error) {
	if len(publicAddress) == 0 {
		return nil, apperrors.BadRequest("public address is required")
	}

	return s.Solscan.GetTokenAccounts(ctx, publicAddress, req)
}

func (s *WalletService) FindUserWalletTokens(
	ctx context.Context,
	opts *repo.Options,
	userID uuid.UUID,
) ([]model.WalletToken, error) {
	tokens, err := s.WalletTokenRepository.FindTokens(ctx, opts, userID)
	if err != nil {
		return nil, apperrors.Internal("failed to find user wallet tokens", err)
	}

	return tokens, nil
}

func (s *WalletService) AddTokenToUserWallet(
	ctx context.Context,
	token *model.WalletToken,
) error {
	if _, err := solana.PublicKeyFromBase58(token.Mint); err != nil {
		return apperrors.BadRequest("failed to decode mint public key", err)
	}

	err := s.WalletTokenRepository.Create(ctx, token)
	if err != nil {
		if repo.DuplicateKeyViolation(err) {
			return apperrors.AlreadyExist("token already added to user's wallet", err)
		}

		return apperrors.Internal("failed to add token to user wallet", err)
	}

	return nil
}

func (s *WalletService) EditUserWalletToken(
	ctx context.Context,
	userID uuid.UUID,
	token *model.WalletTokenEditReq,
) error {
	if err := s.WalletTokenRepository.Update(ctx, userID, token); err != nil {
		return apperrors.Internal("failed to edit user wallet token", err)
	}

	return nil
}

func (s *WalletService) RemoveTokenFromUserWallet(
	ctx context.Context,
	userID uuid.UUID,
	mint string,
) error {
	if err := s.WalletTokenRepository.Delete(ctx, userID, mint); err != nil {
		return apperrors.Internal("failed to edit user wallet token", err)
	}

	return nil
}

func (s *WalletService) GetAllTxNotifications(
	ctx context.Context,
	userID uuid.UUID,
) ([]model.TxNotification, error) {
	notifications, err := s.TxNotificationStorage.GetAll(ctx, userID)
	if err != nil {
		return nil, apperrors.Internal("failed to edit user wallet token", err)
	}

	return notifications, nil
}

func (s *WalletService) DeleteTxNotifications(
	ctx context.Context,
	userID uuid.UUID,
	notificationIDs uuid.UUIDs,
) error {
	if len(notificationIDs) == 0 {
		return apperrors.BadRequest("invalid request data: no ids provided")
	}

	return s.TxNotificationStorage.Delete(ctx, userID, notificationIDs)
}
