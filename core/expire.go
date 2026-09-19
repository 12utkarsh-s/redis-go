package core

import (
	"time"
)

func hasExpired(obj *Object) bool {
	exp, isExpirySet := getExpiry(obj)
	if !isExpirySet {
		return false
	}
	return exp <= uint64(time.Now().UnixMilli())
}

func getExpiry(obj *Object) (uint64, bool) {
	exp, isExpirySet := expires[obj]
	return exp, isExpirySet
}

func expireSample() float32 {
	var limit, expiredCount = 20, 0

	for key, obj := range store {
		if _, isExpirySet := getExpiry(obj); isExpirySet {
			limit--
			if hasExpired(obj) {
				Delete(key)
				expiredCount++
			}
		}
		if limit == 0 {
			break
		}
	}

	return float32(expiredCount) / float32(20.0)
}

// DeleteExpiredKeys Deletes all the expired keys - the active way
// Sampling approach: https://redis.io/commands/expire/
func DeleteExpiredKeys() {
	for {
		frac := expireSample()
		if frac < 0.25 {
			break
		}
	}
}
