package core

import (
	"bytes"
	"fmt"

	"golang.org/x/sys/unix"
)

type Client struct {
	Fd     int
	cqueue RedisCmds
	isTxn  bool
}

func (c *Client) Read(b []byte) (int, error) {
	return unix.Read(c.Fd, b)
}

func (c *Client) Write(b []byte) (int, error) {
	return unix.Write(c.Fd, b)
}

func (c *Client) TxnBegin() {
	c.isTxn = true
}

func (c *Client) TxnQueue(command *RedisCmd) {
	c.cqueue = append(c.cqueue, command)
}

func (c *Client) TxnExec() []byte {
	queue := c.cqueue
	c.resetQueue()

	var out []byte
	buf := bytes.NewBuffer(out)

	buf.WriteString(fmt.Sprintf("*%d\r\n", len(queue)))
	for _, cmd := range queue {
		buf.Write(executeCommand(cmd, c))
	}
	return buf.Bytes()
}

func (c *Client) TxnDiscard() {
	c.resetQueue()
}

func (c *Client) resetQueue() {
	c.cqueue = make(RedisCmds, 0)
	c.isTxn = false
}

func NewClient(fd int) *Client {
	return &Client{
		Fd:     fd,
		cqueue: make(RedisCmds, 0),
	}
}
