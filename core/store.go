package core

import (
	"log"
	"time"
)

type Object struct {
	ExpiresAt int64
	Value     interface{}
}

var store map[string]*Object

func init() {
	store = make(map[string]*Object)
}

func NewObject(value interface{}, duration int64) *Object {
	var expiresAt int64 = -1
	if duration > 0 {
		expiresAt = time.Now().UnixMilli() + duration
	}

	log.Println("Object", value)
	log.Println("ExpiresAt", expiresAt)
	return &Object{
		ExpiresAt: expiresAt,
		Value:     value,
	}
}

func Put(key string, obj *Object) {
	log.Println("Put ", obj.Value, " ", obj.ExpiresAt)
	store[key] = obj
}

func Get(key string) *Object {
	return store[key]
}
