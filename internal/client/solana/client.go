package solana

import (
	"github.com/gagliardetto/solana-go/rpc"
	"gitlab.com/duel-duck/duel-duck-api/config"
)

func NewClient(c *config.Config) *rpc.Client {
	return rpc.New(c.App.SolanaURL)
}
