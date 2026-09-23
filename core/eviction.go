package core

import (
	"math/rand"
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
	case "allkeys-lfu":
		evictAllkeysLFU()
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

// evictionClock is one reading of both clocks the strategies rank on. A batch
// of comparisons shares a single reading so the ordering stays a valid total
// order even if time ticks while a sort is running.
type evictionClock struct {
	seconds uint32
	minutes uint32
}

func newEvictionClock() evictionClock {
	return evictionClock{
		seconds: getCurrentClock(),
		minutes: getCurrentClockInMinutes(),
	}
}

// evictionScore ranks an object as an eviction candidate - the higher the
// score, the sooner it should go. LRU scores by idle time; LFU inverts the
// decayed access counter so that rarely used keys score highest. Both
// strategies therefore share one pool ordering.
func evictionScore(clock evictionClock, lruBits uint32) uint32 {
	if config.EvictionStrategy == "allkeys-lfu" {
		return 255 - lfuDecrAt(clock.minutes, lruBits)
	}
	return getIdleTimeAt(clock.seconds, lruBits)
}

func populateEvictionPool() {
	sampleSize := 5
	for k := range store {
		ePool.Push(k, store[k].LRUBits)
		sampleSize--
		if sampleSize <= 0 {
			break
		}
	}
}

func evictAllkeysLRU() {
	evictBySampling()
}

// evictBySampling is the sampling half of the approximated LRU and LFU
// algorithms. Both differ only in how a candidate is ranked, which the pool
// ordering already covers, so they share the eviction loop itself.
func evictBySampling() {
	evictCount := int64(config.EvictionRatio * float64(config.KeysLimit))

	// One sample only yields a handful of candidates, so keep sampling into the
	// pool until the target is met. A round that evicts nothing necessarily
	// drains the pool, and the next populate then refills it from a non-empty
	// store, so each round makes progress.
	for evicted := int64(0); evicted < evictCount && len(store) > 0; {
		populateEvictionPool()

		// Evict only the single best candidate before sampling again. Draining
		// the pool here would evict everything sampled regardless of rank,
		// which is just random eviction - the pool has to outlive a round so
		// that the stalest keys accumulate in it.
		//
		// A pooled item is only a snapshot taken when the key was sampled, so
		// it is re-validated against the store before we act on it. Rejected
		// items are not counted towards evictCount - they freed nothing.
		for {
			item := ePool.Pop()
			if item == nil {
				break
			}

			obj, ok := store[item.key]
			if !ok {
				// Deleted by DEL, expiry or an earlier eviction since sampling.
				continue
			}
			if obj.LRUBits != item.lruBits {
				// Accessed since being sampled, so its idle time - or its
				// access counter - is no longer what the pool ranked it on.
				continue
			}

			Delete(item.key)
			evicted++
			break
		}
	}
}

/*
The approximated LFU algorithm

An object's LRUBits carries both halves of the LFU state: the upper 16 bits
hold the minute at which it was last decayed (the LDT), the lower 8 bits a
logarithmic access counter. The counter saturates at 255, so it tracks the
order of magnitude of a key's access rate rather than an exact count.
*/

func getCurrentClockInMinutes() uint32 {
	return uint32(time.Now().Unix()/60) & 65535
}

func minutesElapsedAt(nowInMinutes uint32, ldt uint32) uint32 {
	if nowInMinutes >= ldt {
		return nowInMinutes - ldt
	}
	return 65535 - ldt + nowInMinutes
}

func updateLFU(object *Object) {
	counter := lfuDecr(object.LRUBits)
	counter = lfuLogIncr(counter)
	object.LRUBits = (getCurrentClockInMinutes() << 8) | counter
}

func lfuDecr(lruBits uint32) uint32 {
	return lfuDecrAt(getCurrentClockInMinutes(), lruBits)
}

// lfuDecrAt is lfuDecr against a caller-supplied clock. It only returns the
// decayed counter - the decay is folded back into the object by updateLFU, so
// ranking a key never mutates it.
func lfuDecrAt(nowInMinutes uint32, lruBits uint32) uint32 {
	counter := lruBits & 0xFF
	ldt := lruBits >> 8
	if config.LfuDecayTime == 0 {
		return counter
	}
	numPeriods := minutesElapsedAt(nowInMinutes, ldt) / config.LfuDecayTime
	if numPeriods >= counter {
		return 0
	}
	return counter - numPeriods
}

// lfuLogIncr increments the counter with probability 1/((counter-init)*factor+1),
// so a key needs exponentially more accesses to climb each further step and a
// burst of hits cannot pin a key in the store.
func lfuLogIncr(counter uint32) uint32 {
	if counter == 255 {
		return 255
	}
	base := float64(counter) - config.LfuInitVal
	if base < 0 {
		base = 0
	}
	p := 1.0 / (base*config.LfuLogFactor + 1)
	if rand.Float64() < p {
		counter++
	}
	return counter
}

func evictAllkeysLFU() {
	evictBySampling()
}
