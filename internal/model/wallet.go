package model

import (
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"time"
)

// Wallet is deprecated
type Wallet struct {
	UserID    uuid.UUID `bun:",pk,notnull,type:uuid" json:"user_id"`
	Address   string    `bun:",unique,notnull,type:varchar(44)" json:"address"`
	Name      string    `bun:",notnull,type:varchar(255)" json:"name"`
	CreatedAt time.Time `bun:",nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:",nullzero,default:current_timestamp" json:"updated_at"`
}

func NewWallet(userID uuid.UUID, pubAddress, walletName string) *Wallet {
	now := time.Now().UTC()
	return &Wallet{
		UserID:    userID,
		Address:   pubAddress,
		Name:      walletName,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

const (
	TransactionTypeDuelPrediction uint8 = 1
	TransactionTypeDuelRefund     uint8 = 2
	TransactionTypeDuelCommission uint8 = 3
	TransactionTypeDuelReward     uint8 = 4
)

type TransactionType struct {
	bun.BaseModel `bun:"table:transactions,alias:tx" json:"-"`

	Signature string `bun:"signature,pk,type:CHAR(88)" json:"signature"`
	TxType    uint8  `bun:"tx_type,type:SMALLINT,notnull" json:"tx_type"`
}

func NewTransaction(txType uint8, signature string) TransactionType {
	return TransactionType{Signature: signature, TxType: txType}
}

func NewTransactionsWithSameType(txType uint8, signatures ...string) []TransactionType {
	txs := make([]TransactionType, 0, len(signatures)+1)

	for _, signature := range signatures {
		txs = append(txs, NewTransaction(txType, signature))
	}

	return txs
}

type GetTransactionTypesReq struct {
	Signatures []string `json:"signatures"`
}

type CreateWalletResp struct {
	EncryptedMnemonic string `json:"encrypted_mnemonic"`
	PublicAddress     string `json:"public_address"`
}

type TransferReq struct {
	Recipient string `json:"recipient"`
	Amount    uint64 `json:"amount"`
}

type ShowWalletReq struct {
	PublicCypherKey string `json:"public_cypher_key"`
}

type ContractServiceTxResp struct {
	RawTx []byte `json:"raw_tx"`
}

type TokenInfo struct {
	Mint                 string      `json:"mint"`
	Standard             string      `json:"standard"`
	Name                 string      `json:"name"`
	Symbol               string      `json:"symbol"`
	Logo                 interface{} `json:"logo"`
	Decimals             string      `json:"decimals"`
	TotalSupply          string      `json:"totalSupply"`
	TotalSupplyFormatted string      `json:"totalSupplyFormatted"`
	FullyDilutedValue    string      `json:"fullyDilutedValue"`
	Metaplex             Metaplex    `json:"metaplex"`
}

type Metaplex struct {
	MetadataUri          string `json:"metadataUri"`
	MasterEdition        bool   `json:"masterEdition"`
	IsMutable            bool   `json:"isMutable"`
	PrimarySaleHappened  int    `json:"primarySaleHappened"`
	SellerFeeBasisPoints int    `json:"sellerFeeBasisPoints"`
	UpdateAuthority      string `json:"updateAuthority"`
}

type SwapReq struct {
	InputMint  string  `json:"input_mint"`
	OutputMint string  `json:"output_mint"`
	Amount     float64 `json:"amount"`
}
