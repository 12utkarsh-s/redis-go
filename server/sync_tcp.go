package server

import (
	"fmt"
	"io"
	"log"
	"net"
	"redis-go/config"
	"redis-go/core"
	"strconv"
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

func respond(cmds []*core.RedisCmd, c io.ReadWriter) {
	core.EvalAndRespond(cmds, c)
}

func RunSyncTCPServer() {
	log.Println("starting a synchronous TCP server on", config.Host, config.Port)

	var conClients = 0

	// listening to the configured host:port
	lsnr, err := net.Listen("tcp", config.Host+":"+strconv.Itoa(config.Port))
	if err != nil {
		panic(err)
	}

	for {
		// blocking call: waiting for the new client to connect
		c, err := lsnr.Accept()
		if err != nil {
			panic(err)
		}

		// increment the number of concurrent clients
		conClients += 1
		log.Println("client connected with address:", c.RemoteAddr(), ", concurrent clients", conClients)

		for {
			// over the socket, continuously read the command and print it out
			cmd, err := readCommands(c)
			if err != nil {
				c.Close()
				conClients -= 1
				log.Println("client disconnected", c.RemoteAddr(), "concurrent clients", conClients)
				if err == io.EOF {
					break
				}
				log.Println("err", err)
			}
			respond(cmd, c)
		}
	}
}
