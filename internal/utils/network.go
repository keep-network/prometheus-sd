package utils

import (
	"net"
	"strconv"
	"time"
)

func IsPortOpen(protocol, hostname string, port int, scanPortTimeout time.Duration) bool {
	address := hostname + ":" + strconv.Itoa(port)
	conn, err := net.DialTimeout(protocol, address, scanPortTimeout)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}
