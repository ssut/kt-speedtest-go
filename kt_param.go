package main

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	ktParamPatternSessionID      = regexp.MustCompile(`SESSIONID=(?P<sessionid>[\d]+)`)
	ktParamPatternBuffer         = regexp.MustCompile(`BUFFER=(?P<buffer>[\d]+)`)
	ktParamPatternPacketOverhead = regexp.MustCompile(`OVERHEAD=(?P<overhead>[\d]+)`)
	ktParamPatternChunk          = regexp.MustCompile(`CHUNK=(?P<chunk>[\d]+)`)
	ktParamPatternPorts          = regexp.MustCompile(`PORT="(?P<ports>[\d,]+)"`)
	ktParamPatternTime           = regexp.MustCompile(`TIME=(?P<time>[\d]+)`)
	ktParamPatternBytes          = regexp.MustCompile(`BYTES=(?P<bytes>[\d]+)`)
	ktParamPatternKbps           = regexp.MustCompile(`KBPS=(?P<kbps>[\d.]+)`)
)

type KTParam struct {
	SessionID      string
	Buffer         int
	Ports          []int
	PacketOverhead int
	Chunk          int

	Time  int
	Bytes int64
	Kbps  float64
}

func parseIntParam(pattern *regexp.Regexp, message string) int {
	if submatches := pattern.FindStringSubmatch(message); len(submatches) > 1 {
		if i, err := strconv.Atoi(submatches[1]); err == nil {
			return i
		}
	}

	return 0
}

func parseInt64Param(pattern *regexp.Regexp, message string) int64 {
	if submatches := pattern.FindStringSubmatch(message); len(submatches) > 1 {
		if i, err := strconv.ParseInt(submatches[1], 10, 64); err == nil {
			return i
		}
	}

	return 0
}

func parseFloat64Param(pattern *regexp.Regexp, message string) float64 {
	if submatches := pattern.FindStringSubmatch(message); len(submatches) > 1 {
		if i, err := strconv.ParseFloat(submatches[1], 64); err == nil {
			return i
		}
	}

	return 0
}

func ParseParamMessage(message string) KTParam {
	ktParam := KTParam{}

	if message == "" || !strings.Contains(message, ":") || !strings.HasSuffix(message, ";") {
		return ktParam
	}

	if sessionIDSubmatches := ktParamPatternSessionID.FindStringSubmatch(message); len(sessionIDSubmatches) > 1 {
		ktParam.SessionID = sessionIDSubmatches[1]
	}
	ktParam.Buffer = parseIntParam(ktParamPatternBuffer, message)
	ktParam.PacketOverhead = parseIntParam(ktParamPatternPacketOverhead, message)
	ktParam.Chunk = parseIntParam(ktParamPatternChunk, message)
	if portsSubmatches := ktParamPatternPorts.FindStringSubmatch(message); len(portsSubmatches) > 1 {
		ports := strings.Split(portsSubmatches[1], ",")
		for _, port := range ports {
			if i, err := strconv.Atoi(port); err == nil && i > 0 && i < 65536 {
				ktParam.Ports = append(ktParam.Ports, i)
			}
		}
	}
	ktParam.Time = parseIntParam(ktParamPatternTime, message)
	ktParam.Bytes = parseInt64Param(ktParamPatternBytes, message)
	ktParam.Kbps = parseFloat64Param(ktParamPatternKbps, message)

	return ktParam
}
