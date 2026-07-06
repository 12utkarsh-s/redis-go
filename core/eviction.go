package core

func evictFirst() {
	for key := range store {
		delete(store, key)
		return
	}
}

func Evict() {
	evictFirst()
}
