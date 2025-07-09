package solana

import (
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
