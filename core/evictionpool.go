package core

import "sort"

type PoolItem struct {
	key string
	// lruBits is the object's LRUBits as it stood when the key was sampled -
	// an idle-time clock under LRU, a decay stamp plus access counter under
	// LFU. evictionScore is what reads it either way.
	lruBits uint32
}

type EvictionPool struct {
	pool   []*PoolItem
	keyset map[string]*PoolItem
}

// ByEvictionScore orders items best-candidate first, by whichever score the
// configured strategy ranks on. The clock is captured once and held for the
// whole sort, so the ordering stays a valid total order even if the clock
// ticks while sort.Sort is running.
type ByEvictionScore struct {
	items []*PoolItem
	clock evictionClock
}

func byEvictionScore(items []*PoolItem) ByEvictionScore {
	return ByEvictionScore{items: items, clock: newEvictionClock()}
}

func (a ByEvictionScore) Len() int {
	return len(a.items)
}

func (a ByEvictionScore) Swap(i, j int) {
	a.items[i], a.items[j] = a.items[j], a.items[i]
}

func (a ByEvictionScore) Less(i, j int) bool {
	return evictionScore(a.clock, a.items[i].lruBits) >
		evictionScore(a.clock, a.items[j].lruBits)
}

func (pq *EvictionPool) Push(key string, lruBits uint32) {
	_, ok := pq.keyset[key]
	if ok {
		return
	}

	if len(pq.pool) < ePoolSizeMax {
		item := &PoolItem{key: key, lruBits: lruBits}
		pq.keyset[key] = item
		pq.pool = append(pq.pool, item)
		sort.Sort(byEvictionScore(pq.pool))
	} else if clock := newEvictionClock(); evictionScore(clock, lruBits) > evictionScore(clock, pq.pool[len(pq.pool)-1].lruBits) {
		item := &PoolItem{key: key, lruBits: lruBits}
		toRemove := pq.pool[len(pq.pool)-1]
		pq.pool = pq.pool[:len(pq.pool)-1]
		delete(pq.keyset, toRemove.key)

		pq.keyset[key] = item
		pq.pool = append(pq.pool, item)
		sort.Sort(byEvictionScore(pq.pool))
	}
}

func (pq *EvictionPool) Pop() *PoolItem {
	if len(pq.pool) == 0 {
		return nil
	}
	item := pq.pool[0]
	pq.pool = pq.pool[1:]
	delete(pq.keyset, item.key)
	return item
}

func newEvictionPool(size int) *EvictionPool {
	return &EvictionPool{
		pool:   make([]*PoolItem, 0, size),
		keyset: make(map[string]*PoolItem),
	}
}

var ePoolSizeMax int = 16
var ePool *EvictionPool = newEvictionPool(0)
