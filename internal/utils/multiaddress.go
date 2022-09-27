package utils

import (
	"fmt"
	"regexp"
	"strconv"
)

func ExtractAddressFromMultiAddress(multiAddress string) (string, int, error) {
	pattern := regexp.MustCompile(`^\/(?P<code>.+)\/(?P<address>.+)\/(?P<protocol>.+)\/(?P<port>\d+)`)

	m := pattern.FindStringSubmatch(multiAddress)
	if len(m) == 0 {
		return "", 0, fmt.Errorf("failed to find string submatch")
	}
	result := make(map[string]string)
	for i, name := range pattern.SubexpNames() {
		if i != 0 && name != "" {
			result[name] = m[i]
		}
	}

	peerAddress := result["address"]
	if peerAddress == "" {
		return "", 0, fmt.Errorf("extracted peer address is empty")
	}

	peerNetworkPort, err := strconv.Atoi(result["port"])
	if err != nil || peerNetworkPort == 0 {
		return "", 0, fmt.Errorf("failed to extract network port: %v", err)
	}

	return peerAddress, peerNetworkPort, nil
}
