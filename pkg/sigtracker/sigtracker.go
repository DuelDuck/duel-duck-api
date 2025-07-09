package sigtracker

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	solanalib "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	solana "github.com/gagliardetto/solana-go/rpc"
)

type TxTracker struct {
	pendingSignatures   []solanalib.Signature
	confirmedSignatures map[solanalib.Signature]bool

	client *rpc.Client
	mu     sync.RWMutex
}

func NewTransactionTracker(client *solana.Client) *TxTracker {
	return &TxTracker{
		client:              client,
		confirmedSignatures: make(map[solanalib.Signature]bool),
	}
}

func (t *TxTracker) Start() {
	go t.confirmSignatures()
}

func (t *TxTracker) SubscribeForSignatureStatus(sig solanalib.Signature, timeout time.Duration) (bool, error) {
	t.mu.Lock()
	t.pendingSignatures = append(t.pendingSignatures, sig)
	t.mu.Unlock()

	return t.NotifyForSignatureStatus(sig, timeout)
}

func (t *TxTracker) confirmSignatures() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			t.mu.Lock()
			if len(t.pendingSignatures) == 0 {
				t.mu.Unlock()
				return
			}

			// fmt.Println("GOT LEN", len(t.pendingSignatures))
			signatures := slices.Clone(t.pendingSignatures)

			// fmt.Println("TRACKING", signatures)

			result, err := t.client.GetSignatureStatuses(
				ctx,
				true,
				signatures...,
			)
			if err != nil {
				fmt.Println("ERROR", err)
				t.mu.Unlock()
				return
			}

			t.pendingSignatures = t.pendingSignatures[:0]

			t.mu.Unlock()

			if len(result.Value) != len(signatures) {
				fmt.Println("some signatures are not received")
				return
			}

			var newPendingSignatures []solanalib.Signature
			for i, sig := range signatures {
				if result.Value[i] == nil {
					newPendingSignatures = append(newPendingSignatures, sig)
					continue
				}

				// spew.Dump(result.Value[i])

				if result.Value[i].Err != nil {
					t.confirmedSignatures[sig] = false
					continue
				}

				statuses := []rpc.ConfirmationStatusType{
					rpc.ConfirmationStatusFinalized,
					rpc.ConfirmationStatusConfirmed,
					// rpc.ConfirmationStatusProcessed,
				}

				if slices.Contains(statuses, result.Value[i].ConfirmationStatus) {
					t.confirmedSignatures[sig] = true
				} else {
					// Keep unconfirmed signatures in the pending list
					newPendingSignatures = append(newPendingSignatures, sig)
				}
			}

			// Update the pendingSignatures list
			t.mu.Lock()
			t.pendingSignatures = newPendingSignatures
			t.mu.Unlock()
		}()
	}
}

func (t *TxTracker) NotifyForSignatureStatus(
	signature solanalib.Signature,
	timeout time.Duration,
) (bool, error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	timeoutChan := time.After(timeout)

	for {
		select {
		case <-ticker.C:

			t.mu.RLock()
			status, exist := t.confirmedSignatures[signature]
			t.mu.RUnlock()

			if exist {
				t.mu.Lock()
				delete(t.confirmedSignatures, signature)
				t.mu.Unlock()
				return status, nil
			}
		case <-timeoutChan:
			return false, fmt.Errorf("signature %s: %w", signature, rpc.ErrNotConfirmed)
		}
	}
}
