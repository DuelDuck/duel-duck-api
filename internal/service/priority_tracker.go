package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gitlab.com/duel-duck/duel-duck-api/config"
	"io"
	"net/http"
	"sync/atomic"
	"time"
)

// TODO: move priority tracker to package clients

type PriorityTracker struct {
	mediumPriority *atomic.Uint64
	highPriority   *atomic.Uint64

	updateInterval    time.Duration
	updatePriorityReq []byte
	JSONRpcURL        string
}

func (t *PriorityTracker) GetHighPriorityMicroLamports() uint64 {
	return t.highPriority.Load()
}

func (t *PriorityTracker) GetMediumPriorityMicroLamports() uint64 {
	return t.mediumPriority.Load()
}

type GetPriorityDataParams struct {
	Account     *string `json:"account"`
	LastNBlocks int     `json:"last_n_blocks"`
	ApiVersion  int     `json:"api_version"`
}

type GetPriorityData struct {
	JsonRPC string                `json:"jsonrpc"`
	ID      int                   `json:"id"`
	Method  string                `json:"method"`
	Params  GetPriorityDataParams `json:"params"`
}

type GetPriorityDataResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  struct {
		Context struct {
			Slot int `json:"slot"`
		} `json:"context"`
		PerComputeUnit struct {
			Extreme uint64 `json:"extreme"`
			High    uint64 `json:"high"`
			Low     uint64 `json:"low"`
			Medium  uint64 `json:"medium"`
		} `json:"per_compute_unit"`
		//PerTransaction struct {
		//	Extreme uint64 `json:"extreme"`
		//	High    uint64 `json:"high"`
		//	Low     uint64 `json:"low"`
		//	Medium  uint64 `json:"medium"`
		//} `json:"per_transaction"`
	} `json:"result"`
	ID int `json:"id"`
}

func (t *PriorityTracker) UpdatePriority() error {
	resp, err := http.Post(t.JSONRpcURL, "application/json", bytes.NewReader(t.updatePriorityReq))
	if err != nil {
		return fmt.Errorf("could not send json payload: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("invalid response code: %d, body: %s", resp.StatusCode, string(body))
	}

	var response GetPriorityDataResponse
	if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("could not decode json payload: %w", err)
	}

	cu := response.Result.PerComputeUnit

	if cu.High == 0 || cu.Medium == 0 {
		return fmt.Errorf("could not update priority: invalid response: Result.PerComputeUnits are 0: %#v", response)
	}

	t.mediumPriority.Store(cu.Medium)
	t.highPriority.Store(cu.High)

	return nil
}

const (
	FallbackMicroLamports uint64 = 1_273_683
)

func NewPriorityTracker(c *config.Config) (*PriorityTracker, error) {
	updatePriorityReq, err := json.Marshal(GetPriorityData{
		JsonRPC: "2.0",
		ID:      1,
		Method:  "qn_estimatePriorityFees",
		Params: GetPriorityDataParams{
			LastNBlocks: 100,
			ApiVersion:  2,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not marshal json payload: %w", err)
	}

	t := &PriorityTracker{
		mediumPriority:    new(atomic.Uint64),
		highPriority:      new(atomic.Uint64),
		JSONRpcURL:        c.App.SolanaQuickNodeAPI,
		updatePriorityReq: updatePriorityReq,
	}

	t.mediumPriority.Store(FallbackMicroLamports)
	t.highPriority.Store(FallbackMicroLamports)

	if err = t.UpdatePriority(); err != nil {
		return nil, fmt.Errorf("could not update priority stats: %w", err)
	}

	return t, nil
}
