package main

import (
	"fmt"
)

const (
	defaultControlPort    = 10011
	defaultPort           = 10021
	defaultBufferSize     = 1048576
	maxBufferForMac       = 4194304
	waitingTimeout        = 15000
	requestTimeout        = 3000
	mtuSize               = 1500
	defaultPacketOverhead = 0
)

func checkIP() {

}

type KTSpeed struct {
	controlClient *controlClient
}

func main() {
	cc := NewControlClient("121.156.34.195", 10011)
	ktSpeed := &KTSpeed{
		controlClient: &cc,
	}

	if err := ktSpeed.controlClient.Connect(); err != nil {
		panic(err)
	}

	publicIP, err := ktSpeed.controlClient.CheckPublicIP()
	if err != nil {
		panic(err)
	}
	realIP, err := ktSpeed.controlClient.CheckRealIP()
	if err != nil {
		panic(err)
	}
	ktSpeed.controlClient.Close()
	fmt.Printf("Public IP: %s\nReal IP: %s\n", publicIP, realIP)

	ktdl := NewKTSpeedMeter(realIP, KTSpeedMeterDefaultDownloadPort, KTSpeedMeterDownload)
	if err := ktdl.Connect(); err != nil {
		panic(err)
	}
	fmt.Println("connected to download server")
	ktdl.Measure()

	ktup := NewKTSpeedMeter(realIP, KTSpeedMeterDefaultUploadPort, KTSpeedMeterUpload)
	if err := ktup.Connect(); err != nil {
		panic(err)
	}
	fmt.Println("connected to upload server")
	ktup.Measure()

}
