package core

import "sort"

type PoolItem struct {
	key            string
	lastAccessedAt uint32
}

type EvictionPool struct {
	pool   []*PoolItem
	keyset map[string]*PoolItem
}

// ByIdleTime orders items most-idle first. The clock is captured once and held
// for the whole sort, so the ordering stays a valid total order even if the
// clock ticks while sort.Sort is running.
type ByIdleTime struct {
	items []*PoolItem
	now   uint32
}

func byIdleTime(items []*PoolItem) ByIdleTime {
	return ByIdleTime{items: items, now: getCurrentClock()}
}

func (a ByIdleTime) Len() int {
	return len(a.items)
}

func (a ByIdleTime) Swap(i, j int) {
	a.items[i], a.items[j] = a.items[j], a.items[i]
}

func (a ByIdleTime) Less(i, j int) bool {
	return getIdleTimeAt(a.now, a.items[i].lastAccessedAt) >
		getIdleTimeAt(a.now, a.items[j].lastAccessedAt)
}

func (pq *EvictionPool) Push(key string, lastAccessedAt uint32) {
	_, ok := pq.keyset[key]
	if ok {
		return
	}

	if len(pq.pool) < ePoolSizeMax {
		item := &PoolItem{key: key, lastAccessedAt: lastAccessedAt}
		pq.keyset[key] = item
		pq.pool = append(pq.pool, item)
		sort.Sort(byIdleTime(pq.pool))
	} else if now := getCurrentClock(); getIdleTimeAt(now, lastAccessedAt) > getIdleTimeAt(now, pq.pool[len(pq.pool)-1].lastAccessedAt) {
		item := &PoolItem{key: key, lastAccessedAt: lastAccessedAt}
		toRemove := pq.pool[len(pq.pool)-1]
		pq.pool = pq.pool[:len(pq.pool)-1]
		delete(pq.keyset, toRemove.key)

		pq.keyset[key] = item
		pq.pool = append(pq.pool, item)
		sort.Sort(byIdleTime(pq.pool))
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
