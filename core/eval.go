package core

import (
	"bytes"
	"errors"
	"io"
	"log"
	"strconv"
	"strings"
	"time"
)

var RespNil = []byte("$-1\r\n")
var RespOk = []byte("+OK\r\n")
var RespZero = []byte(":0\r\n")
var RespOne = []byte(":1\r\n")
var RespMinus1 = []byte(":-1\r\n")
var RespMinus2 = []byte(":-2\r\n")

func evalPING(args []string) []byte {
	if len(args) >= 2 {
		return Encode(errors.New("ERR wrong number of arguments for 'ping' command"), false)
	}

	var resp []byte
	if len(args) == 0 {
		resp = Encode("PONG", true)
	} else {
		resp = Encode(args[0], false)
	}

	return resp
}

func evalSET(args []string) []byte {
	if len(args) < 2 {
		return Encode(errors.New("ERR wrong number of arguments for 'set' command"), false)
	}

	var key, value string
	var durationInMs int64 = -1

	key, value = args[0], args[1]

	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "EX", "ex":
			i += 1
			if i >= len(args) {
				return Encode(errors.New("ERR syntax error"), false)
			}

			duration, err := strconv.ParseInt(args[i], 10, 64)
			if err != nil {
				return Encode(errors.New("ERR value is not a number or out of range"), false)
			}
			durationInMs = duration * 1000
		default:
			return Encode(errors.New("ERR syntax error"), false)
		}

	}

	Put(key, NewObject(value, durationInMs))
	return RespOk
}

func evalGET(args []string) []byte {
	if len(args) != 1 {
		return Encode(errors.New("ERR wrong number of arguments for 'get' command"), false)
	}

	var key = args[0]

	obj := Get(key)

	if obj == nil {
		return RespNil
	}

	if obj.ExpiresAt != -1 && obj.ExpiresAt <= time.Now().UnixMilli() {
		return RespNil
	}

	return Encode(obj.Value, false)
}

func evalTTL(args []string) []byte {
	if len(args) != 1 {
		return Encode(errors.New("ERR wrong number of arguments for 'ttl' command"), false)
	}

	var key = args[0]

	obj := Get(key)
	if obj == nil {
		return RespMinus2
	}

	if obj.ExpiresAt == -1 {
		return RespMinus1
	}

	durationLeft := obj.ExpiresAt - time.Now().UnixMilli()
	if durationLeft < 0 {
		return RespMinus2
	}

	return Encode(durationLeft/1000, false)
}

func evalDEL(args []string) []byte {
	var countDeleted = 0

	for _, key := range args {
		if ok := Delete(key); ok {
			countDeleted++
		}
	}

	return Encode(countDeleted, false)
}

func evalEXPIRE(args []string) []byte {
	if len(args) != 2 {
		return Encode(errors.New("ERR wrong number of arguments for 'expire' command"), false)
	}

	var key = args[0]

	exDurationSex, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return Encode(errors.New("ERR value is not a number or out of range"), false)
	}

	obj := Get(key)
	if obj == nil {
		return RespZero
	}

	obj.ExpiresAt = time.Now().UnixMilli() + (exDurationSex * 1000)

	return RespOne
}

func EvalAndRespond(cmds []*RedisCmd, c io.ReadWriter) {

	var response []byte
	buf := bytes.NewBuffer(response)

	for _, cmd := range cmds {
		command := strings.ToUpper(cmd.Cmd)
		switch command {
		case "PING":
			buf.Write(evalPING(cmd.Args))
		case "SET":
			buf.Write(evalSET(cmd.Args))
		case "GET":
			buf.Write(evalGET(cmd.Args))
		case "TTL":
			buf.Write(evalTTL(cmd.Args))
		case "DEL":
			buf.Write(evalDEL(cmd.Args))
		case "EXPIRE":
			buf.Write(evalEXPIRE(cmd.Args))
		default:
			buf.Write(evalPING(cmd.Args))
		}
	}

	log.Println("here")
	c.Write(buf.Bytes())
}
