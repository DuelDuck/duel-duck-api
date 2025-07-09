package cache

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gitlab.com/duel-duck/duel-duck-api/config"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"io"
	"time"
)

type WalletCacheStorage struct {
	client        *redis.Client
	cacheTTL      time.Duration
	encryptionKey []byte
}

func NewWalletCacheStorage(
	c *config.Config,
	client *redis.Client,
) *WalletCacheStorage {
	return &WalletCacheStorage{
		client:        client,
		cacheTTL:      c.App.WalletCacheTTL,
		encryptionKey: []byte(c.App.WalletCacheEncryptionKey),
	}
}

func (s *WalletCacheStorage) Save(ctx context.Context, userID uuid.UUID, privateKey string) error {
	encryptedPK, err := s.encrypt(privateKey)
	if err != nil {
		return apperrors.Internal("failed to encrypt caching wallet data", err)
	}

	return s.save(ctx, userID, encryptedPK)
}

func (s *WalletCacheStorage) save(ctx context.Context, userID uuid.UUID, privateKey string) error {
	key := userID.String()
	err := s.client.Set(ctx, key, privateKey, s.cacheTTL).Err()
	if err != nil {
		return apperrors.Internal("failed to cache encrypted wallet data", err)
	}

	return nil
}

func (s *WalletCacheStorage) GetWalletByUserID(ctx context.Context, userID uuid.UUID) (string, error) {
	encryptedPK, err := s.getWalletByUserID(ctx, userID)
	if err != nil {
		return "", err
	}

	privateKey, err := s.decrypt(encryptedPK)
	if err != nil {
		return "", apperrors.Internal("failed to decrypt caching wallet data", err)
	}

	return privateKey, nil
}

func (s *WalletCacheStorage) getWalletByUserID(ctx context.Context, userID uuid.UUID) (string, error) {
	privateKey, err := s.client.Get(ctx, userID.String()).Result()
	if err != nil {
		return "", apperrors.NotFound("failed to find wallet data in cache", err)
	}

	if privateKey == "" {
		return "", apperrors.NotFound("wallet data not found in cache")
	}

	return privateKey, nil
}

func (s *WalletCacheStorage) DeleteWalletByUserID(ctx context.Context, userID uuid.UUID) error {
	_, err := s.client.Del(ctx, userID.String()).Result()
	if err != nil {
		return apperrors.Internal("failed to delete wallet from cache", err)
	}

	return nil
}

func (s *WalletCacheStorage) encrypt(privateKey string) (string, error) {
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", apperrors.Internal("failed to encrypt wallet data", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", apperrors.Internal("failed to encrypt wallet data", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", apperrors.Internal("wallet caching: failed to generate nonce", err)
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(privateKey), nil)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (s *WalletCacheStorage) decrypt(encodedCiphertext string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encodedCiphertext)
	if err != nil {
		return "", apperrors.Internal("failed to decode cached wallet data", err)
	}

	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", apperrors.Internal("failed to decrypt cached wallet data", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", apperrors.Internal("failed to decrypt cached wallet data", err)

	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", apperrors.Internal("failed to decrypt cached wallet data: nonce is too short", err)
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	privateKey, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", apperrors.Internal("failed to decrypt cached wallet data", err)

	}

	return string(privateKey), nil
}
