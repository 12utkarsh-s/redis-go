package core

import "redis-go/config"

func evictFirst() {
	for key := range store {
		Delete(key)
		return
	}
}

func evictAllkeysRandom() {
	evictCount := int64(config.EvictionRatio * float64(config.KeysLimit))

	for key := range store {
		Delete(key)
		evictCount--
		if evictCount <= 0 {
			break
		}
	}
}

func Evict() {
	switch config.EvictionStrategy {
	case "simple-first":
		evictFirst()
	case "allkeys-random":
		evictAllkeysRandom()
	}
	evictFirst()
}
