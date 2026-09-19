package core

import (
	"redis-go/config"
	"time"
)

func Evict() {
	switch config.EvictionStrategy {
	case "simple-first":
		evictFirst()
	case "allkeys-random":
		evictAllkeysRandom()
	case "allkeys-lru":
		evictAllkeysLRU()
	}
}

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

/*
The approximated LRU algorithm
*/

func getCurrentClock() uint32 {
	return uint32(time.Now().Unix()) & 0x00FFFFFF
}

func getIdleTime(lastAccessedAt uint32) uint32 {
	return getIdleTimeAt(getCurrentClock(), lastAccessedAt)
}

// getIdleTimeAt is getIdleTime against a caller-supplied clock, so a batch of
// comparisons can share one reading instead of re-sampling time per call.
func getIdleTimeAt(currentTime uint32, lastAccessedAt uint32) uint32 {
	if currentTime >= lastAccessedAt {
		return currentTime - lastAccessedAt
	}
	return (0x00FFFFFF - lastAccessedAt) + currentTime
}

func populateEvictionPool() {
	sampleSize := 5
	for k := range store {
		ePool.Push(k, store[k].LastAccessedAt)
		sampleSize--
		if sampleSize <= 0 {
			break
		}
	}
}

func evictAllkeysLRU() {
	evictCount := int64(config.EvictionRatio * float64(config.KeysLimit))

	// One sample only yields a handful of candidates, so keep sampling into the
	// pool until the target is met. A round that evicts nothing necessarily
	// drains the pool, and the next populate then refills it from a non-empty
	// store, so each round makes progress.
	for evicted := int64(0); evicted < evictCount && len(store) > 0; {
		populateEvictionPool()

		// A pooled item is only a snapshot taken when the key was sampled, so it
		// is re-validated against the store before we act on it. Rejected items
		// are not counted towards evictCount - they freed nothing.
		for evicted < evictCount {
			item := ePool.Pop()
			if item == nil {
				break
			}

			obj, ok := store[item.key]
			if !ok {
				// Deleted by DEL, expiry or an earlier eviction since sampling.
				continue
			}
			if obj.LastAccessedAt != item.lastAccessedAt {
				// Accessed since being sampled, so its idle time is no longer
				// what the pool ranked it on.
				continue
			}

			Delete(item.key)
			evicted++
		}
	}
}
