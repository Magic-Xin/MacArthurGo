package essentials

import (
	"MacArthurGo/structs"
	"sync"
	"time"
)

type EchoCache struct {
	Value structs.IncomingMessageStruct
	Time  int64
}

func init() {
	go DeleteExpiredCache(3600, 1800)
}

var cache sync.Map

func SetCache(key string, value EchoCache) {
	cache.Store(key, value)
}

func GetCache(key string) (value any, ok bool) {
	return cache.Load(key)
}

func DeleteExpiredCache(expiration int64, interval int64) {
	for {
		cache.Range(func(key, value any) bool {
			if time.Now().Unix()-value.(EchoCache).Time > expiration {
				cache.Delete(key)
			}
			return true
		})
		time.Sleep(time.Duration(interval) * time.Second)
	}
}
