package core

import (
	"strconv"
)

func deduceTypeAndEncoding(val string) (uint8, uint8) {
	var oType = OBJ_TYPE_STRING

	if _, err := strconv.ParseInt(val, 10, 64); err == nil {
		return oType, OBJ_ENCODING_INT
	}
	if len(val) <= 44 {
		return oType, OBJ_ENCODING_EMBSTR
	}
	return oType, OBJ_ENCODING_RAW
}
