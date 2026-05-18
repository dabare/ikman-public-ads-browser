package ikman

import (
	"sync"
	"time"
)

type cache struct {
	mu    sync.Mutex
	items map[string]cacheItem
}

type cacheItem struct {
	body      []byte
	expiresAt time.Time
}

func newCache() *cache {
	return &cache{items: map[string]cacheItem{}}
}

func (c *cache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	item, ok := c.items[key]
	if !ok || time.Now().After(item.expiresAt) {
		if ok {
			delete(c.items, key)
		}
		return nil, false
	}
	body := make([]byte, len(item.body))
	copy(body, item.body)
	return body, true
}

func (c *cache) set(key string, body []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	copyBody := make([]byte, len(body))
	copy(copyBody, body)
	c.items[key] = cacheItem{body: copyBody, expiresAt: time.Now().Add(ttl)}
}

func (c *cache) delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

type rateLimiter struct {
	mu       sync.Mutex
	next     time.Time
	interval time.Duration
}

func newRateLimiter(interval time.Duration) *rateLimiter {
	if interval < 0 {
		interval = 0
	}
	return &rateLimiter{interval: interval}
}

func (r *rateLimiter) wait() {
	if r.interval == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	if now.Before(r.next) {
		time.Sleep(time.Until(r.next))
	}
	r.next = time.Now().Add(r.interval)
}
