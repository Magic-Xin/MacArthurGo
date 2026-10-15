package essentials

import (
	"MacArthurGo/structs"
	"context"
	"sync"
	"time"
)

type EchoCache struct {
	Value structs.MessageStruct
	Time  int64
}

var cache sync.Map

func SetCache(key string, value EchoCache) {
	cache.Store(key, value)
}

func GetCache(key string) (EchoCache, bool) {
	value, ok := cache.Load(key)
	if !ok {
		return EchoCache{}, false
	}
	entry, ok := value.(EchoCache)
	return entry, ok
}

func StartCacheJanitor(ctx context.Context, expiration, interval time.Duration) {
	if expiration <= 0 {
		expiration = time.Hour
	}
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				deleteExpiredCache(expiration)
			}
		}
	}()
}

func deleteExpiredCache(expiration time.Duration) {
	cutoff := time.Now().Add(-expiration).Unix()
	cache.Range(func(key, value any) bool {
		entry, ok := value.(EchoCache)
		if !ok || entry.Time < cutoff {
			cache.Delete(key)
		}
		return true
	})
}
