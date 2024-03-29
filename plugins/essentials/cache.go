package essentials

import (
	"sync"
	"time"
)

type Value struct {
	Value any
	Time  int64
}

func init() {
	go DeleteExpiredCache(3600, 1800)

}

var cache sync.Map

func SetCache(key string, value Value) {
	cache.Store(key, value)
}

func GetCache(key string) (value any, ok bool) {
	return cache.Load(key)
}

func DeleteExpiredCache(expiration int64, interval int64) {
	for {
		cache.Range(func(key, value any) bool {
			if time.Now().Unix()-value.(Value).Time > expiration {
				cache.Delete(key)
			}
			return true
		})
		time.Sleep(time.Duration(interval) * time.Second)
	}
}
