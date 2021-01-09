package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"strings"
)

type controlClient struct {
	IP   string
	Port int

	conn   *net.TCPConn
	reader *bufio.Reader
}

func NewControlClient(ip string, port int) controlClient {
	cc := controlClient{
		IP:   ip,
		Port: port,
	}

	return cc
}

func (c *controlClient) readLine() (string, error) {
	line, _, err := c.reader.ReadLine()
	if err != nil {
		return "", err
	}
	lineStr := strings.TrimSpace(string(line))

	return lineStr, nil
}

func (c *controlClient) send(command string) (string, error) {
	if _, err := c.conn.Write([]byte(fmt.Sprintf("%s\n", command))); err != nil {
		return "", err
	}

	lineStr, err := c.readLine()
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(lineStr, "CONNECT OK!") {
		lineStr, err = c.readLine()
		if err != nil {
			return "", err
		}
	}
	if strings.HasPrefix(lineStr, "NOK") {
		return "", fmt.Errorf("NOK: %s", lineStr)
	}

	var buffer bytes.Buffer
	for {
		lineStr, err := c.readLine()
		if err == nil {
			if lineStr[len(lineStr)-1] == ';' {
				break
			}

			buffer.Write([]byte(lineStr))
		}
	}

	return buffer.String(), nil
}

func (c *controlClient) Connect() error {
	addr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", c.IP, c.Port))
	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		return err
	}
	c.conn = conn
	c.reader = bufio.NewReader(c.conn)
	return nil
}

func (c *controlClient) Close() error {
	return c.conn.Close()
}

func (c *controlClient) CheckPublicIP() (string, error) {
	return c.send("RTRV-MY-IP:;")
}

func (c *controlClient) CheckRealIP() (string, error) {
	return c.send("RTRV-REAL-IP:;")
}
