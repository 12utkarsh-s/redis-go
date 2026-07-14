package core

import (
	"redis-go/config"
	"time"
)

type Object struct {
	Value        interface{}
	TypeEncoding uint8
	ExpiresAt    int64
}

var store map[string]*Object

func init() {
	store = make(map[string]*Object)
}

func NewObject(value interface{}, duration int64, oType uint8, oEnc uint8) *Object {
	var expiresAt int64 = -1
	if duration > 0 {
		expiresAt = time.Now().UnixMilli() + duration
	}

	return &Object{
		Value:        value,
		TypeEncoding: oType | oEnc,
		ExpiresAt:    expiresAt,
	}
}

func Put(key string, obj *Object) {
	if len(store) >= config.KeysLimit {
		Evict()
	}
	store[key] = obj
}

func Get(key string) *Object {
	obj, _ := store[key]
	if obj != nil && obj.ExpiresAt != -1 && obj.ExpiresAt <= time.Now().UnixMilli() {
		delete(store, key)
		return nil
	}

	return obj
}

func Delete(key string) bool {
	if _, ok := store[key]; ok {
		delete(store, key)
		return true
	}
	return false
}
