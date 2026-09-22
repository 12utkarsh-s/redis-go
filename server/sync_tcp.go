package server

import (
	"fmt"
	"io"
	"redis-go/core"
)

func toArrayString(values []interface{}) ([]string, error) {
	var tokens = make([]string, 0)
	for i := range values {
		tokens = append(tokens, fmt.Sprintf("%v", values[i]))
	}

	return tokens, nil
}

func readCommands(c io.ReadWriter) (core.RedisCmds, error) {
	// TODO: Max read in one shot is 512 bytes
	// To allow input > 512 bytes, then repeated read until
	// we get EOF or designated delimiter
	var buf = make([]byte, 512)
	n, err := c.Read(buf[:])
	if err != nil {
		return nil, err
	}

	values, err := core.Decode(buf[:n])
	if err != nil {
		return nil, err
	}

	var cmds = make([]*core.RedisCmd, 0)
	for _, value := range values {
		tokens, err := toArrayString(value.([]interface{}))
		if err != nil {
			return nil, err
		}
		cmds = append(cmds, &core.RedisCmd{
			Cmd:  tokens[0],
			Args: tokens[1:],
		})
	}

	return cmds, nil
}

func respondError(err error, c io.ReadWriter) {
	c.Write([]byte(fmt.Sprintf("-%s\r\n", err)))
}

func respond(cmds []*core.RedisCmd, c *core.Client) {
	core.EvalAndRespond(cmds, c)
}
