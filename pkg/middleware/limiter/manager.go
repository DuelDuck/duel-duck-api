package limiter

import (
	"github.com/gofiber/fiber/v3"
	"gitlab.com/duel-duck/duel-duck-api/pkg/middleware/limiter/memory"
	"sync"
	"time"
)

type reqData struct {
	currHits int
	prevHits int
	exp      uint64
}

//msgp:ignore manager
type manager struct {
	pool    sync.Pool
	memory  *memory.Storage
	storage fiber.Storage
}

func newManager() *manager {
	// Create new storage handler
	manager := &manager{
		pool: sync.Pool{
			New: func() any {
				return new(reqData)
			},
		},
	}

	manager.memory = memory.New(5 * time.Second)

	return manager
}

// acquire returns an *entry from the sync.Pool
func (m *manager) acquire() *reqData {
	return m.pool.Get().(*reqData) //nolint:forcetypeassert,errcheck // We store nothing else in the pool
}

// release and reset *entry to sync.Pool
func (m *manager) release(e *reqData) {
	e.prevHits = 0
	e.currHits = 0
	e.exp = 0
	m.pool.Put(e)
}

// get data from storage or memory
func (m *manager) get(key string) *reqData {
	var it *reqData

	if it, _ = m.memory.Get(key).(*reqData); it == nil { //nolint:errcheck // We store nothing else in the pool
		it = m.acquire()
		return it
	}

	return it
}

// set data to storage or memory
func (m *manager) set(key string, it *reqData, exp time.Duration) {
	m.memory.Set(key, it, exp)
}
