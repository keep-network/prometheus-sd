package utils

import (
	"testing"

	"golang.org/x/exp/slices"
)

func TestSortAddresses(t *testing.T) {
	addresses := []string{
		"127.0.0.1",
		"bootstrap-1.test.keep.network",
		"10.102.2.4",
		"34.141.9.57",
		"bootstrap-2.test.keep.network",
		"0-node.test.keep.network",
	}
	expectedAddress := []string{
		"0-node.test.keep.network",
		"bootstrap-1.test.keep.network",
		"bootstrap-2.test.keep.network",
		"34.141.9.57",
		"127.0.0.1",
		"10.102.2.4",
	}

	sortedAddresses := SortAddresses(addresses)

	if slices.Compare(expectedAddress, sortedAddresses) != 0 {
		t.Errorf(
			"invalid address\nexpected: %s\nactual:   %s",
			expectedAddress,
			sortedAddresses,
		)
	}
}
