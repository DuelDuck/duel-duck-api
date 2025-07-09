package memory

import (
	"github.com/gofiber/utils/v2"
	"sync"
	"time"
)

type Storage struct {
	data map[string]item // data
	sync.RWMutex
}

type item struct {
	value any
	exp   uint32
}

func New(expDuration time.Duration) *Storage {
	store := &Storage{
		data: make(map[string]item),
	}

	utils.StartTimeStampUpdater()
	go store.gc(expDuration)

	return store
}

// Get value by key
func (s *Storage) Get(key string) any {
	s.RLock()
	v, ok := s.data[key]
	s.RUnlock()
	if !ok || v.exp != 0 && v.exp <= utils.Timestamp() {
		return nil
	}
	return v.value
}

// Set key with value
func (s *Storage) Set(key string, val any, ttl time.Duration) {
	var exp uint32
	if ttl > 0 {
		exp = uint32(ttl.Seconds()) + utils.Timestamp()
	}
	i := item{exp: exp, value: val}
	s.Lock()
	s.data[key] = i
	s.Unlock()
}

// Delete key by key
func (s *Storage) Delete(key string) {
	s.Lock()
	delete(s.data, key)
	s.Unlock()
}

// Reset all keys
func (s *Storage) Reset() {
	nd := make(map[string]item)
	s.Lock()
	s.data = nd
	s.Unlock()
}

func (s *Storage) gc(sleep time.Duration) {
	ticker := time.NewTicker(sleep)
	defer ticker.Stop()
	var expired []string

	for range ticker.C {
		ts := utils.Timestamp()
		expired = expired[:0]
		s.RLock()
		for key, v := range s.data {
			if v.exp != 0 && v.exp <= ts {
				expired = append(expired, key)
			}
		}
		s.RUnlock()
		s.Lock()
		// Double-checked locking.
		// We might have replaced the item in the meantime.
		for i := range expired {
			v := s.data[expired[i]]
			if v.exp != 0 && v.exp <= ts {
				delete(s.data, expired[i])
			}
		}
		s.Unlock()
	}
}
