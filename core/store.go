package core

import (
	"redis-go/config"
	"time"
)

var store map[string]*Object
var expires map[*Object]uint64

func init() {
	store = make(map[string]*Object)
	expires = make(map[*Object]uint64)
}

func setExpiry(object *Object, expDurationMs int64) {
	expires[object] = uint64(time.Now().UnixMilli()) + uint64(expDurationMs)
}

func NewObject(value interface{}, duration int64, oType uint8, oEnc uint8) *Object {
	obj := &Object{
		Value:          value,
		TypeEncoding:   oType | oEnc,
		LastAccessedAt: getCurrentClock(),
	}
	if duration > 0 {
		setExpiry(obj, duration)
	}
	return obj
}

func Put(key string, obj *Object) {
	if len(store) >= config.KeysLimit {
		Evict()
	}
	obj.LastAccessedAt = getCurrentClock()
	if KeySpaceStats[0] == nil {
		KeySpaceStats[0] = make(map[string]int)
	}
	KeySpaceStats[0]["keys"]++
	store[key] = obj
}

func Get(key string) *Object {
	obj, ok := store[key]
	if !ok {
		return nil
	}
	if hasExpired(obj) {
		Delete(key)
		return nil
	}
	obj.LastAccessedAt = getCurrentClock()
	return obj
}

func Delete(key string) bool {
	if obj, ok := store[key]; ok {
		delete(store, key)
		delete(expires, obj)
		KeySpaceStats[0]["keys"]--
		return true
	}
	return false
}
