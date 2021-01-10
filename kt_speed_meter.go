package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"runtime"
	"strings"
	"time"
)

type KTSpeedMeterMeasureType int

const (
	KTSpeedMeterDownload KTSpeedMeterMeasureType = iota
	KTSpeedMeterUpload
)

const (
	KTSpeedMeterDefaultDownloadPort = 10021
	KTSpeedMeterDefaultUploadPort   = 10031
)

type KTSpeedMeter struct {
	IP   string
	Port int
	Type KTSpeedMeterMeasureType

	TestCount         int
	TimeSlice         time.Duration
	TestTimeout       time.Duration
	WaitingTimeout    time.Duration
	ConnectionTimeout time.Duration
	RequestTimeout    time.Duration

	mtuSize        int
	conn           *net.TCPConn
	reader         *bufio.Reader
	params         *KTParam
	trafficCounter *TrafficCounter
}

func NewKTSpeedMeter(ip string, port int, measureType KTSpeedMeterMeasureType) KTSpeedMeter {
	ktSpeedMeter := KTSpeedMeter{
		IP:                ip,
		Port:              port,
		Type:              measureType,
		TestCount:         20,
		TimeSlice:         500 * time.Millisecond,
		TestTimeout:       time.Duration(float64(500*time.Millisecond*20) * 2.5),
		ConnectionTimeout: 3 * time.Second,
		RequestTimeout:    3 * time.Second,
		WaitingTimeout:    15 * time.Second,
		mtuSize:           1460,
	}

	return ktSpeedMeter
}

func (k *KTSpeedMeter) Connect() error {
	addr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", k.IP, k.Port))
	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		return err
	}
	k.conn = conn
	k.reader = bufio.NewReader(k.conn)
	return nil
}

func (k *KTSpeedMeter) Measure() ([]int64, error) {
	ks := NewKTStream(k.reader)
	recv, err := ks.ReadMessage(k.WaitingTimeout)
	if err != nil {
		return nil, err
	}
	if recv != "READY;" {
		return nil, fmt.Errorf("Unexpected response. Expected=[READY;], Received=[%s]", recv)
	}

	k.conn.Write([]byte("AUTH-REQ:;"))
	recv, err = ks.ReadMessage(k.WaitingTimeout)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(recv, "AUTH-OK") {
		return nil, fmt.Errorf("Unexpected response. Expected=[AUTH-OK:...], Received=[%s]", recv)
	}
	params := ParseParamMessage(recv)
	fmt.Printf("%+v\n", params)
	if runtime.GOOS != "windows" && params.Buffer > 4194304 {
		params.Buffer = 4194304
	}
	k.params = &params

	childRunners := make([]KTSpeedMeterRunner, len(k.params.Ports))
	for i, port := range k.params.Ports {
		childRunners[i] = NewKTSpeedMeterRunner(k, k.IP, port)
		if err := childRunners[i].Prepare(); err != nil {
			return nil, err
		}
	}

	tc := NewTrafficCounter()
	k.trafficCounter = &tc
	nChan, closeTrafficCounter := k.trafficCounter.Run()
	ctx, cancel := context.WithCancel(context.Background())
	for _, cr := range childRunners {
		go cr.Run(ctx, nChan)
	}

	prevTime := time.Now().Nanosecond()
	var totalBytes int64
	var realTimeMax int64
	var realTimeTotal int64
	k.conn.Write([]byte("START;"))
	for i := 0; i < k.TestCount; i++ {
		count := i + 1
		var currentBytes int64
		var realTime int

		if k.Type == KTSpeedMeterDownload {
			time.Sleep(k.TimeSlice)

			currentBytes = tc.GetInterval()
			now := time.Now().Nanosecond()
			realTime = (now - prevTime) / 1000000
			prevTime = now
			if realTime <= 0 {
				realTime = 500
			}
		} else if k.Type == KTSpeedMeterUpload {
			recv, err := ks.ReadMessage(k.RequestTimeout)
			if err != nil {
				return nil, err
			}
			if !strings.HasPrefix(recv, "DATA") {
				return nil, fmt.Errorf("Unexpected response. Expected=[DATA:...], Received=[%s]", recv)
			}

			data := ParseParamMessage(recv)
			currentBytes = data.Bytes
			realTime = data.Time
		}

		totalBytes += currentBytes
		realTimeTotal += int64(realTime)
		if realTimeMax < int64(realTime) {
			realTimeMax = int64(realTime)
		}

		bps := currentBytes * 1000 * 8 / int64(realTime)
		d := float64(realTime) / float64(k.TimeSlice.Milliseconds())
		if d < 0.9 || d > 1.1 {
			fmt.Printf("### Abnormal real time slice:: realTime=%dms (d=%.2f)\n", realTime, d)
		}
		log := fmt.Sprintf("%02d  bytes=%9d,  Mbps=%7.2f,  Duration=%d", count, currentBytes, float64(bps)/1000000.0, realTime)
		fmt.Println(log)
	}
	k.conn.Write([]byte("STOP;"))
	totalDuration := realTimeTotal
	mbps := float64(totalBytes) * 1000.0 * 8.0 / float64(totalDuration) / 1000000.0
	fmt.Printf("total test time=%dms\n", totalDuration)
	fmt.Printf("total bytes=%d, Mbps=%d\n", totalBytes, int(float64(mbps)*100.0/100.0))
	cancel()
	closeTrafficCounter()

	return []int64{}, nil
}

type KTSpeedMeterRunner struct {
	IP   string
	Port int

	meter  *KTSpeedMeter
	conn   *net.TCPConn
	reader *bufio.Reader

	chunk []byte
}

func NewKTSpeedMeterRunner(meter *KTSpeedMeter, ip string, port int) KTSpeedMeterRunner {
	cr := KTSpeedMeterRunner{
		IP:    ip,
		Port:  port,
		meter: meter,
	}
	return cr
}

func (r *KTSpeedMeterRunner) Prepare() error {
	addr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", r.IP, r.Port))
	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		return err
	}
	r.conn = conn
	r.reader = bufio.NewReader(conn)
	if err := r.conn.SetNoDelay(true); err != nil {
		return err
	}
	if r.meter.Type == KTSpeedMeterUpload {
		// if err := r.conn.SetWriteBuffer(128 * 1024); err != nil {
		// 	fmt.Println(err)
		// }
	}

	ks := NewKTStream(r.reader)
	recv, err := ks.ReadMessage(r.meter.ConnectionTimeout)
	if err != nil {
		return err
	}
	if recv != "READY;" {
		return fmt.Errorf("Unexpected response. Expected=[READY;], Received=[%s]", recv)
	}

	r.conn.Write([]byte(fmt.Sprintf("AUTH-REQ:SESSIONID=%s;", r.meter.params.SessionID)))
	recv, err = ks.ReadMessage(r.meter.WaitingTimeout)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(recv, "AUTH-OK;") {
		return fmt.Errorf("Unexpected response. Expected=[AUTH-OK;], Received=[%s]", recv)
	}

	if r.meter.Type == KTSpeedMeterUpload {
		chunk := make([]byte, r.meter.params.Chunk, r.meter.params.Chunk)
		for i := 0; i < r.meter.params.Chunk; i++ {
			chunk[i] = 65
		}
		r.chunk = chunk
	}

	return nil
}

func (r *KTSpeedMeterRunner) Run(ctx context.Context, nChan chan<- int64) {
	mtuSize := int64(r.meter.mtuSize)
	packetOverhead := int64(r.meter.params.PacketOverhead)
	go func() {
		for {
			<-ctx.Done()
			r.conn.Write([]byte("STOP;"))
			r.conn.Close()
			return
		}
	}()

	if r.meter.Type == KTSpeedMeterDownload {
		buf := make([]byte, 14599)
		r.conn.Write([]byte("START;"))
		for {
			size, err := r.reader.Read(buf) // cr.conn.Read(buf)
			if err != nil {
				r.conn.Close()
				break
			}

			size += int(float64(packetOverhead) * (float64(size)/float64(mtuSize) + 1.0))
			nChan <- int64(size)
		}
	} else if r.meter.Type == KTSpeedMeterUpload {
		r.conn.Write([]byte("START;"))
		for {
			r.conn.Write(r.chunk)
		}
	}

}
