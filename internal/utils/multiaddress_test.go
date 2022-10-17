package utils

import (
	"testing"
)

func TestExtractAddressFromMultiAddress(t *testing.T) {
	var tests = map[string]struct {
		multiaddress    string
		expectedAddress string
		expectedPort    int
	}{
		"dns4": {
			multiaddress:    "/dns4/bootstrap-1.test.keep.network/tcp/3919",
			expectedAddress: "bootstrap-1.test.keep.network",
			expectedPort:    3919,
		},
		"ip4": {
			multiaddress:    "/ip4/10.102.4.6/tcp/45861/",
			expectedAddress: "10.102.4.6",
			expectedPort:    45861,
		},
		"ip6": {
			multiaddress:    "/ip6/2604:1380:2000:7a00::1/tcp/4001",
			expectedAddress: "2604:1380:2000:7a00::1",
			expectedPort:    4001,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			actualAddress, actualPort, err := ExtractAddressFromMultiAddress(test.multiaddress)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if test.expectedAddress != actualAddress {
				t.Errorf("invalid address\nexpected: %s\nactual:   %s", test.expectedAddress, actualAddress)
			}

			if test.expectedPort != actualPort {
				t.Errorf("invalid port\nexpected: %d\nactual:   %d", test.expectedPort, actualPort)
			}
		})
	}
}
