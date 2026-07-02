package wireguard

import (
	"fmt"
	"net/netip"
)

// PrefixLen returns the CIDR prefix length of a subnet like "10.50.0.0/24".
func PrefixLen(subnet string) (int, error) {
	pfx, err := netip.ParsePrefix(subnet)
	if err != nil {
		return 0, fmt.Errorf("parse subnet %q: %w", subnet, err)
	}
	return pfx.Bits(), nil
}

// WithPrefix appends a subnet's prefix length to a bare address, yielding a
// value suitable for an Interface Address (e.g. "10.50.0.2" -> "10.50.0.2/24").
func WithPrefix(addr, subnet string) (string, error) {
	bits, err := PrefixLen(subnet)
	if err != nil {
		return "", err
	}
	if _, err := netip.ParseAddr(addr); err != nil {
		return "", fmt.Errorf("parse address %q: %w", addr, err)
	}
	return fmt.Sprintf("%s/%d", addr, bits), nil
}

// NextClientAddress returns the lowest host address within subnet that is not
// already in taken. The network address (.0 for IPv4) is skipped. This supports
// adding further clients without a schema change.
func NextClientAddress(subnet string, taken []string) (string, error) {
	pfx, err := netip.ParsePrefix(subnet)
	if err != nil {
		return "", fmt.Errorf("parse subnet %q: %w", subnet, err)
	}
	pfx = pfx.Masked()

	used := make(map[netip.Addr]bool, len(taken))
	for _, t := range taken {
		if a, err := netip.ParseAddr(t); err == nil {
			used[a] = true
		}
	}

	// Start at the first host address (skip the network address).
	addr := pfx.Addr().Next()
	for pfx.Contains(addr) {
		if !used[addr] {
			return addr.String(), nil
		}
		addr = addr.Next()
	}
	return "", fmt.Errorf("no free address available in %s", subnet)
}
