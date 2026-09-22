package core

import (
	"bytes"
	"errors"
	"fmt"
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
var RespQueued = []byte("+QUEUED\r\n")

var txnCommands map[string]bool

func init() {
	txnCommands = map[string]bool{"MULTI": true, "EXEC": true, "DISCARD": true}
}

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
	oType, oEnc := deduceTypeAndEncoding(value)

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

	Put(key, NewObject(value, durationInMs, oType, oEnc))
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

	exp, isExpirySet := expires[obj]
	if !isExpirySet {
		return RespMinus1
	}

	if hasExpired(obj) {
		return RespMinus2
	}

	durationLeft := exp - uint64(time.Now().UnixMilli())

	return Encode(durationLeft/1000, false)
}

func evalDEL(args []string) []byte {
	if len(args) == 0 {
		return Encode(errors.New("ERR wrong number of arguments for 'del' command"), false)
	}

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

	setExpiry(obj, exDurationSex*1000)

	return RespOne
}

func evalINCR(args []string) []byte {
	if len(args) != 1 {
		return Encode(errors.New("ERR wrong number of arguments for 'incr' command"), false)
	}

	var key = args[0]
	obj := Get(key)
	if obj == nil {
		obj = NewObject("0", -1, OBJ_TYPE_STRING, OBJ_ENCODING_INT)
		Put(key, obj)
	}

	if err := assertType(obj.TypeEncoding, OBJ_TYPE_STRING); err != nil {
		return Encode(err, false)
	}

	if err := assertEncoding(obj.TypeEncoding, OBJ_ENCODING_INT); err != nil {
		return Encode(err, false)
	}

	val, _ := strconv.ParseInt(obj.Value.(string), 10, 64)
	val++
	obj.Value = strconv.FormatInt(val, 10)

	return Encode(val, false)
}

func evalINFO(args []string) []byte {
	var info []byte
	buf := bytes.NewBuffer(info)
	buf.WriteString("# Keyspace\r\n")

	for i := range KeySpaceStats {
		buf.WriteString(fmt.Sprintf("db%d:keys=%d,expires=0,avg_ttl=0\r\n", i, KeySpaceStats[i]["keys"]))
	}
	return Encode(buf.String(), false)
}

func evalCLIENT(args []string) []byte {
	return RespOk
}

func evalLATENCY(args []string) []byte {
	return Encode([]string{}, false)
}

func evalBGREWRITEAOF(args []string) []byte {
	DumpAllAOF()
	return RespOk
}

func evalLRU(args []string) []byte {
	evictAllkeysLRU()
	return RespOk
}

func evalSLEEP(args []string) []byte {
	if len(args) != 1 {
		return Encode(errors.New("ERR wrong number of arguments for 'SLEEP' command"), false)
	}

	durationSec, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return Encode(errors.New("ERR value is not an integer or out of range"), false)
	}
	time.Sleep(time.Duration(durationSec) * time.Second)
	return RespOk
}

func evalMULTI(args []string) []byte {
	return RespOk
}

func executeCommand(cmd *RedisCmd, c *Client) []byte {
	command := strings.ToUpper(cmd.Cmd)
	switch command {
	case "PING":
		return evalPING(cmd.Args)
	case "SET":
		return evalSET(cmd.Args)
	case "GET":
		return evalGET(cmd.Args)
	case "TTL":
		return evalTTL(cmd.Args)
	case "DEL":
		return evalDEL(cmd.Args)
	case "EXPIRE":
		return evalEXPIRE(cmd.Args)
	case "INCR":
		return evalINCR(cmd.Args)
	case "INFO":
		return evalINFO(cmd.Args)
	case "CLIENT":
		return evalCLIENT(cmd.Args)
	case "LATENCY":
		return evalLATENCY(cmd.Args)
	case "BGREWRITEAOF":
		return evalBGREWRITEAOF(cmd.Args)
	case "LRU":
		return evalLRU(cmd.Args)
	case "SLEEP":
		return evalSLEEP(cmd.Args)
	case "MULTI":
		if c.isTxn {
			return Encode(errors.New("ERR MULTI calls can not be nested"), false)
		}
		c.TxnBegin()
		return evalMULTI(cmd.Args)
	case "EXEC":
		if !c.isTxn {
			return Encode(errors.New("ERR EXEC without MULTI"), false)
		}
		return c.TxnExec()
	case "DISCARD":
		if !c.isTxn {
			return Encode(errors.New("ERR DISCARD without MULTI"), false)
		}
		c.TxnDiscard()
		return RespOk
	default:
		return evalPING(cmd.Args)
	}
}

func EvalAndRespond(cmds []*RedisCmd, c *Client) {

	var response []byte
	buf := bytes.NewBuffer(response)

	for _, cmd := range cmds {
		if !c.isTxn {
			buf.Write(executeCommand(cmd, c))
			continue
		} else {
			if !txnCommands[strings.ToUpper(cmd.Cmd)] {
				c.TxnQueue(cmd)
				buf.Write(RespQueued)
			} else {
				buf.Write(executeCommand(cmd, c))
			}
		}
	}

	c.Write(buf.Bytes())
}
