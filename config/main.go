package config

import "time"

var Host = "0.0.0.0"
var Port = 7379

var ExpiryCronFrequency = 1 * time.Second

var KeysLimit = 100
var EvictionRatio = 0.40
var EvictionStrategy = "allkeys-lru"

var AOFFile = "./redis-go.aof"
