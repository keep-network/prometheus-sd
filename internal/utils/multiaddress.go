package utils

import (
	"fmt"
	"regexp"
)

func ExtractAddressFromMultiAddress(multiAddress string) (string, error) {
	pattern := regexp.MustCompile(`^\/(?P<code>.+)\/(?P<address>.+)\/(?P<protocol>.+)\/(?P<port>\d+?)`)

	m := pattern.FindStringSubmatch(multiAddress)
	if len(m) == 0 {
		return "", fmt.Errorf("failed to find string submatch")
	}
	result := make(map[string]string)
	for i, name := range pattern.SubexpNames() {
		if i != 0 && name != "" {
			result[name] = m[i]
		}
	}

	peerAddress := result["address"]
	if peerAddress == "" {
		return "", fmt.Errorf("extracted peer address is empty")
	}

	return peerAddress, nil
}
