package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"time"
)

type ktStream struct {
	Reader *bufio.Reader
}

func NewKTStream(reader *bufio.Reader) ktStream {
	ks := ktStream{
		Reader: reader,
	}
	return ks
}

func (k *ktStream) ticker() <-chan interface{} {
	c := make(chan interface{})
	go func() {
		for {
			c <- struct{}{}
			time.Sleep(1 * time.Millisecond)
		}
	}()
	return c
}

func (k *ktStream) ReadMessage(timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t := k.ticker()

	var buf bytes.Buffer
	for {
		select {
		case <-t:
			if b, err := k.Reader.ReadByte(); err == nil {
				received := int(b)
				if received != -1 {
					buf.WriteByte(b)
					if received != 59 {
						continue
					}
				}

				cancel()
				return buf.String(), nil
			}

		case <-ctx.Done():
			return "", fmt.Errorf("Timeout occurred")
		}
	}
}
