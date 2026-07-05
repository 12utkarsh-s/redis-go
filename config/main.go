package config

import "time"

var Host string = "0.0.0.0"
var Port int = 7379
var ExpiryCronFrequency time.Duration = 1 * time.Second
