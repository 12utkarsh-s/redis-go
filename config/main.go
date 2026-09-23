package config

import "time"

var Host = "0.0.0.0"
var Port = 7379

var ExpiryCronFrequency = 1 * time.Second

var KeysLimit = 1000
var EvictionRatio = 0.40
var EvictionStrategy = "allkeys-lru"

var AOFFile = "./redis-go.aof"

const LfuInitVal = 5
const LfuLogFactor = 10.0
const LfuDecayTime = 1
