package core

var KeySpaceStats [4]map[string]int

func UpdateDBStats(db int, key string, value int) {
	KeySpaceStats[db][key] = value
}
