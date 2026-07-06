package config

import "time"

var Host = "0.0.0.0"
var Port = 7379
var ExpiryCronFrequency = 1 * time.Second
var KeysLimit = 2
