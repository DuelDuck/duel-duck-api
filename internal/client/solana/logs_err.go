package solana

import (
	"errors"
	"github.com/gagliardetto/solana-go/rpc/jsonrpc"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"strings"
)

func ParseLogsForError(logs []string) error {
	for _, log := range logs {
		switch {
		case IsInsufficientFundForCommission(log):
			return apperrors.PaymentRequired("insufficient funds for transaction")
		}
	}

	return apperrors.Internal("transaction: result err")
}

func IsInsufficientFundForCommission(log string) bool {
	return strings.Contains(log, "insufficient")
}

const solanaRPCErrCodeInvalidParams = -32602

func IsTxTooLarge(err error) bool {
	var rpcErr *jsonrpc.RPCError
	if !errors.As(err, &rpcErr) || rpcErr == nil {
		return false
	}

	if rpcErr.Code != solanaRPCErrCodeInvalidParams {
		return false
	}

	return strings.Contains(rpcErr.Message, "too large")

}
