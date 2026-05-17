package memcache

import (
	"encoding/json"
	"sync"

	"github.com/ray-g/dnsproxy/cache"
	r "github.com/ray-g/dnsproxy/cache/record"
)

type MemoryCache struct {
	sync.RWMutex
	Records  map[string]*r.Record `json:"cache"`
	Capacity int                  `json:"capacity"`
}

func NewCache() cache.Cache {
	return NewSizedCache(0)
}

func NewSizedCache(capacity int) cache.Cache {
	return &MemoryCache{
		Records:  make(map[string]*r.Record),
		Capacity: capacity,
	}
}

func (c *MemoryCache) Set(key string, record *r.Record) error {
	if c.Full() {
		return cache.ErrorCacheFull
	}
	if c.Exists(key) {
		return nil
	}
	c.Lock()
	c.Records[key] = record
	c.Unlock()
	return nil
}

func (c *MemoryCache) Get(key string) (record *r.Record, err error) {
	c.RLock()
	record, ok := c.Records[key]
	c.RUnlock()

	if !ok {
		return nil, cache.ErrorCacheKeyMissed
	}
	if record.Expired() {
		c.Remove(key)
		return nil, cache.ErrorCacheKeyExpired
	}
	return record, nil
}

func (c *MemoryCache) Exists(key string) bool {
	c.RLock()
	_, ok := c.Records[key]
	c.RUnlock()
	return ok
}

func (c *MemoryCache) Remove(key string) {
	c.Lock()
	delete(c.Records, key)
	c.Unlock()
}

func (c *MemoryCache) Length() int {
	c.RLock()
	n := len(c.Records)
	c.RUnlock()
	return n
}

func (c *MemoryCache) Full() bool {
	return c.Capacity > 0 && c.Length() >= c.Capacity
}

func (c *MemoryCache) Dump() string {
	c.RLock()
	data, err := json.Marshal(c)
	c.RUnlock()
	if err != nil {
		return "{}"
	}
	return string(data)
}
