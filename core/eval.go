package core

import (
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
)

var RespNil = []byte("$-1\r\n")

func evalPING(args []string, conn io.ReadWriter) error {
	if len(args) >= 2 {
		return errors.New("ERR wrong number of arguments for 'ping' command")
	}

	var resp []byte
	if len(args) == 0 {
		resp = Encode("PONG", true)
	} else {
		resp = Encode(args[0], false)
	}

	_, err := conn.Write(resp)
	return err
}

func evalSET(args []string, conn io.ReadWriter) error {
	if len(args) < 2 {
		return errors.New("ERR wrong number of arguments for 'set' command")
	}

	var key, value string
	var durationInMs int64 = -1

	key, value = args[0], args[1]

	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "EX", "ex":
			i += 1
			if i >= len(args) {
				return errors.New("ERR syntax error")
			}

			duration, err := strconv.ParseInt(args[i], 10, 64)
			if err != nil {
				return errors.New("ERR value is not a number or out of range")
			}
			durationInMs = duration * 1000
		default:
			return errors.New("ERR syntax error")
		}

	}

	Put(key, NewObject(value, durationInMs))
	conn.Write([]byte("+OK\r\n"))
	return nil
}

func evalGET(args []string, conn io.ReadWriter) error {
	if len(args) != 1 {
		return errors.New("ERR wrong number of arguments for 'get' command")
	}

	var key = args[0]

	obj := Get(key)

	if obj == nil {
		conn.Write(RespNil)
		return nil
	}

	if obj.ExpiresAt != -1 && obj.ExpiresAt <= time.Now().UnixMilli() {
		conn.Write(RespNil)
		return nil
	}

	conn.Write(Encode(obj.Value, false))
	return nil
}

func evalTTL(args []string, conn io.ReadWriter) error {
	if len(args) != 1 {
		return errors.New("ERR wrong number of arguments for 'ttl' command")
	}

	var key = args[0]

	obj := Get(key)
	if obj == nil {
		conn.Write([]byte(":-2\r\n"))
		return nil
	}

	if obj.ExpiresAt == -1 {
		conn.Write([]byte(":-1\r\n"))
		return nil
	}

	durationLeft := obj.ExpiresAt - time.Now().UnixMilli()
	if durationLeft < 0 {
		conn.Write([]byte(":-2\r\n"))
		return nil
	}

	conn.Write(Encode(durationLeft/1000, false))
	return nil
}

func evalDEL(args []string, conn io.ReadWriter) error {
	var countDeleted = 0

	for _, key := range args {
		if ok := Delete(key); ok {
			countDeleted++
		}
	}

	conn.Write(Encode(countDeleted, false))
	return nil
}

func evalEXPIRE(args []string, conn io.ReadWriter) error {
	if len(args) != 2 {
		return errors.New("ERR wrong number of arguments for 'expire' command")
	}

	var key = args[0]

	exDurationSex, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return errors.New("ERR value is not a number or out of range")
	}

	obj := Get(key)
	if obj == nil {
		conn.Write([]byte(":0\r\n"))
		return nil
	}

	obj.ExpiresAt = time.Now().UnixMilli() + (exDurationSex * 1000)

	conn.Write([]byte(":1\r\n"))
	return nil
}

func EvalAndRespond(cmd *RedisCmd, c io.ReadWriter) error {
	command := strings.ToUpper(cmd.Cmd)
	switch command {
	case "PING":
		return evalPING(cmd.Args, c)
	case "SET":
		return evalSET(cmd.Args, c)
	case "GET":
		return evalGET(cmd.Args, c)
	case "TTL":
		return evalTTL(cmd.Args, c)
	case "DEL":
		return evalDEL(cmd.Args, c)
	case "EXPIRE":
		return evalEXPIRE(cmd.Args, c)
	default:
		return evalPING(cmd.Args, c)
	}
}
