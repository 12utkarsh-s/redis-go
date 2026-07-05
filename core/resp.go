package core

import (
	"errors"
	"fmt"
)

func readLength(data []byte) (int, int) {
	pos, length := 0, 0
	for pos = range data {
		b := data[pos]
		if !(b >= '0' && b <= '9') {
			return length, pos + 2
		}
		length = length*10 + int(b-'0')
	}
	return 0, 0
}

func readSimpleString(data []byte) (string, int, error) {
	pos := 1

	for ; data[pos] != '\r'; pos++ {
	}

	return string(data[1:pos]), pos + 2, nil
}

func readInt64(data []byte) (int64, int, error) {
	pos := 1
	var value int64 = 0

	for ; data[pos] != '\r'; pos++ {
		value = value*10 + int64(data[pos]-'0')
	}

	return value, pos + 2, nil
}

func readError(data []byte) (string, int, error) {
	return readSimpleString(data)
}

func readBulkString(data []byte) (interface{}, int, error) {
	pos := 1

	length, delta := readLength(data[pos:])
	pos += delta

	return string(data[pos : pos+length]), pos + length + 2, nil
}

func readArray(data []byte) ([]interface{}, int, error) {
	pos := 1

	length, delta := readLength(data[pos:])
	pos += delta

	result := make([]interface{}, length)
	for i := range result {
		value, delta, err := DecodeOne(data[pos:])
		if err != nil {
			return nil, 0, err
		}
		result[i] = value
		pos += delta
	}

	return result, pos, nil
}

func DecodeOne(data []byte) (interface{}, int, error) {
	switch data[0] {
	case '+':
		return readSimpleString(data)
	case ':':
		return readInt64(data)
	case '$':
		return readBulkString(data)
	case '*':
		return readArray(data)
	case '-':
		return readError(data)
	}
	return nil, 0, nil
}

func Decode(data []byte) (interface{}, error) {
	if len(data) == 0 {
		return nil, errors.New("no data")
	}

	value, _, err := DecodeOne(data)
	return value, err
}

func DecodeStringArray(data []byte) ([]string, error) {
	value, err := Decode(data)
	if err != nil {
		return nil, err
	}

	ts := value.([]interface{})
	tokens := make([]string, len(ts))
	for i := range tokens {
		tokens[i] = ts[i].(string)
	}

	return tokens, nil
}

func Encode(data interface{}, isSimple bool) []byte {
	switch v := data.(type) {
	case string:
		if isSimple {
			return []byte(fmt.Sprintf("+%s\r\n", data))
		}
		return []byte(fmt.Sprintf("$%d\r\n%s\r\n", len(v), v))
	case int64:
		return []byte(fmt.Sprintf(":%d\r\n", v))
	default:
		return RespNil
	}
	return []byte{}
}
