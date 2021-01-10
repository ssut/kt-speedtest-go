package main

import (
	"fmt"
	"log"
	"net"
	"time"
)

const (
	KTUDPMeterDefaultPort = 10014
)

type KTUDPMeter struct {
	IP   string
	Port int

	TestCount   int
	TestTimeout time.Duration
	Interval    time.Duration

	conn *net.UDPConn

	receiver       KTUDPMeterReceiver
	sentSequenceID int
	recvOK         bool
}

func setBytesFromInt(buffer []byte, i int) []byte {
	buffer[0] = byte(i >> 24 & 0xFF)
	buffer[1] = byte(i >> 16 & 0xFF)
	buffer[2] = byte(i >> 8 & 0xFF)
	buffer[3] = byte(i & 0xFF)
	return buffer
}

func getIntFromBytes(buffer []byte) int {
	arr := make([]int, len(buffer), len(buffer))
	for i := 0; i < 4; i++ {
		if buffer[i] < 0 {
			arr[i] = int(buffer[i]) + 256
		} else {
			arr[i] = int(buffer[i])
		}
	}

	return (arr[0] << 24) + (arr[1] << 16) + (arr[2] << 8) + arr[3]
}

func NewKTUDPMeter(ip string, port int) KTUDPMeter {
	ktUDPMeter := KTUDPMeter{
		IP:          ip,
		Port:        port,
		TestCount:   100,
		TestTimeout: 15 * time.Second,
		Interval:    100 * time.Millisecond,
	}

	return ktUDPMeter
}

func (k *KTUDPMeter) Measure() ([]float64, error) {
	addr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", k.IP, k.Port))
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	k.conn = conn
	log.Printf("Socket open on %s\n", conn.LocalAddr().String())

	buffer := make([]byte, 32, 32)
	for i := 0; i < 32; i++ {
		buffer[i] = 97
	}

	receiver := NewKTUDPMeterReceiver(k)
	recv := make(chan int, 5)
	go receiver.Run(recv)

	var sentCount int
	var receivedCount int
	rttMin := float64(^uint(0) >> 1)
	rttMax := -1.0
	rttSum := 0.0
	rttAvg := 0.0
	for i := -1; i < k.TestCount; i++ {
		buffer = setBytesFromInt(buffer, i)

		k.sentSequenceID = i
		sentAt := time.Now()
		conn.Write(buffer)
		sentCount++

		rtt := -1.0
		var timeout time.Duration
		if i < 0 {
			timeout = 100 * time.Millisecond
		} else {
			timeout = 500 * time.Millisecond
		}
		timer := time.NewTimer(timeout)
		for {
			var exit bool
			select {
			case <-timer.C:
				exit = true
				fmt.Println("timedout")
				break

			case seq := <-recv:
				if seq == i {
					exit = true

					receivedCount++
					receivedAt := time.Now()

					rtt = (float64(receivedAt.Nanosecond()) - float64(sentAt.Nanosecond())) / float64(time.Millisecond)
					if rtt < float64(rttMin) {
						rttMin = rtt
					}
					if rtt > rttMax {
						rttMax = rtt
					}
					rttSum += rtt
					rttAvg = float64(rttSum) / float64(receivedCount)
				} else {
					rtt = -1.0
					fmt.Println("mismatch")
					continue
				}
			}

			if exit {
				break
			}
		}

		fmt.Printf("RTT = %.2f, AVG = %.2f\n", rtt, rttAvg)

		sleepMillis := int64(k.Interval/time.Millisecond) - int64(rtt)
		time.Sleep(time.Duration(sleepMillis) * time.Millisecond)
	}

	return nil, nil
}

type KTUDPMeterReceiver struct {
	Meter *KTUDPMeter
}

func NewKTUDPMeterReceiver(meter *KTUDPMeter) KTUDPMeterReceiver {
	receiver := KTUDPMeterReceiver{
		Meter: meter,
	}
	return receiver
}

func (r *KTUDPMeterReceiver) Run(seqChan chan<- int) {
	buffer := make([]byte, 1024)
	for {
		n, err := r.Meter.conn.Read(buffer)
		if err != nil {
			return
		}
		if n <= 4 {
			continue
		}

		recvSeqID := getIntFromBytes(buffer)
		seqChan <- recvSeqID
	}
}
