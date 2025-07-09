package cypher

import (
	"context"
	"crypto/ed25519"
	"github.com/goccy/go-json"
	"github.com/google/uuid"
	vault "github.com/hashicorp/vault/api"
	"github.com/mr-tron/base58"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"io"
	"net/http"
)

type PrivateKeyRepository struct {
	client *vault.Client
}

func NewPrivateKeyRepository(client *vault.Client) *PrivateKeyRepository {
	return &PrivateKeyRepository{
		client: client,
	}
}

const privateKeyPath = "secret/data/private_keys/"

func (r *PrivateKeyRepository) WritePrivateKey(
	ctx context.Context,
	userID uuid.UUID,
	mnemonic string,
	privateKey ed25519.PrivateKey,
) error {
	if r.client == nil {
		return apperrors.Internal("failed to initialize the storage client")
	}

	path := privateKeyPath + userID.String()
	privateKeyBase64 := base58.Encode(privateKey)

	data := map[string]any{
		"data": VaultCustomData{
			PrivateKey: privateKeyBase64,
			Mnemonic:   mnemonic,
		},
	}

	logical := r.client.Logical()
	if logical == nil {
		return apperrors.Internal("failed to initialize the storage logical")
	}

	_, err := logical.WriteWithContext(ctx, path, data)
	if err != nil {
		return apperrors.Internal("failed to handle secure private key storing", err)
	}

	return nil
}

type ReadResponse struct {
	Data VaultData `json:"data"`
}

type VaultData struct {
	Data VaultCustomData `json:"data"`
}

type VaultCustomData struct {
	PrivateKey string `json:"private_key"`
	Mnemonic   string `json:"mnemonic"`
}

func (r *PrivateKeyRepository) GetPrivateKeyBase58(ctx context.Context, userID uuid.UUID) (string, error) {
	data, err := r.GetWalletData(ctx, userID)
	if err != nil {
		return "", err
	}

	if data == nil || data.PrivateKey == "" {
		return "", apperrors.Internal("failed to read private key")
	}

	return data.PrivateKey, nil
}

func (r *PrivateKeyRepository) GetWalletData(ctx context.Context, userID uuid.UUID) (*VaultCustomData, error) {
	if r.client == nil {
		return nil, apperrors.Internal("failed to initialize the storage client")
	}

	path := privateKeyPath + userID.String()
	logical := r.client.Logical()
	if logical == nil {
		return nil, apperrors.Internal("failed to initialize the storage logical")
	}

	resp, err := logical.ReadRawWithContext(ctx, path)
	if err != nil {
		return nil, apperrors.Internal("failed to handle secure private key reading", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, apperrors.Internal("failed to read key to sign a transaction")
	}

	readData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.Internal("failed to read resp body", err)
	}

	readResponse := new(ReadResponse)
	if err = json.Unmarshal(readData, readResponse); err != nil || readResponse == nil {
		return nil, apperrors.Internal("failed to read private key", err)
	}

	return &readResponse.Data.Data, nil
}
