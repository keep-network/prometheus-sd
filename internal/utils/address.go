package utils

import (
	"net"
	"sort"
)

// SortAddresses sorts slice of addresses.
func SortAddresses(addresses []string) []string {
	nonIps := make([]string, 0)
	ips := make([]string, 0)

	for _, address := range addresses {
		if ip := net.ParseIP(address); ip != nil {
			ips = append(ips, address)
			continue
		}
		nonIps = append(nonIps, address)
	}

	sort.Strings(nonIps)
	sort.Sort(sort.Reverse(sort.StringSlice(ips)))

	return append(nonIps, ips...)
}
