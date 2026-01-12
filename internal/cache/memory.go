package cache

import (
	"context"
	"sync"
	"time"
)

type MemoryCache struct {
	data   map[string]*cacheItem
	mu     sync.RWMutex
	stopCh chan struct{}
}

type cacheItem struct {
	value     []byte
	expiresAt time.Time
}

func NewMemoryCache() *MemoryCache {
	mc := &MemoryCache{
		data:   make(map[string]*cacheItem),
		stopCh: make(chan struct{}),
	}

	go mc.cleanupExpired()

	return mc
}

func (mc *MemoryCache) Get(ctx context.Context, key string) ([]byte, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	item, exists := mc.data[key]
	if !exists {
		return nil, ErrNotFound
	}

	if time.Now().After(item.expiresAt) {
		return nil, ErrNotFound
	}

	valueCopy := make([]byte, len(item.value))
	copy(valueCopy, item.value)
	return valueCopy, nil
}

func (mc *MemoryCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)

	mc.data[key] = &cacheItem{
		value:     valueCopy,
		expiresAt: time.Now().Add(ttl),
	}

	return nil
}

func (mc *MemoryCache) Delete(ctx context.Context, key string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	delete(mc.data, key)
	return nil
}

func (mc *MemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	item, exists := mc.data[key]
	if !exists {
		return false, nil
	}

	if time.Now().After(item.expiresAt) {
		return false, nil
	}

	return true, nil
}

func (mc *MemoryCache) Close() error {
	close(mc.stopCh)
	return nil
}

func (mc *MemoryCache) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mc.cleanup()
		case <-mc.stopCh:
			return
		}
	}
}

func (mc *MemoryCache) cleanup() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	now := time.Now()
	for key, item := range mc.data {
		if now.After(item.expiresAt) {
			delete(mc.data, key)
		}
	}
}
