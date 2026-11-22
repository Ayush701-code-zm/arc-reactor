package network

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
)

func ParseCIDR(cidr string) (*net.IPNet, error) {
	_, ipNet, err := net.ParseCIDR(strings.TrimSpace(cidr))
	if err != nil {
		return nil, fmt.Errorf("invalid cidr: %w", err)
	}
	ones, bits := ipNet.Mask.Size()
	if bits != 32 || ones > 30 {
		return nil, fmt.Errorf("only IPv4 CIDRs with prefix <= 30 are supported")
	}
	return ipNet, nil
}

func GatewayIP(ipNet *net.IPNet) (net.IP, error) {
	ip := ipNet.IP.To4()
	if ip == nil {
		return nil, fmt.Errorf("ipv4 required")
	}
	base := binary.BigEndian.Uint32(ip) & binary.BigEndian.Uint32(ipNet.Mask)
	gw := make(net.IP, 4)
	binary.BigEndian.PutUint32(gw, base+1)
	if !ipNet.Contains(gw) {
		return nil, fmt.Errorf("gateway not in cidr")
	}
	return gw, nil
}

func AllocateGuestIP(ipNet *net.IPNet, gateway net.IP, used map[string]struct{}) (net.IP, error) {
	ip4 := ipNet.IP.To4()
	mask := ipNet.Mask
	base := binary.BigEndian.Uint32(ip4) & binary.BigEndian.Uint32(mask)
	bcast := base | ^binary.BigEndian.Uint32(mask)
	gw := binary.BigEndian.Uint32(gateway.To4())
	for host := base + 1; host < bcast; host++ {
		if host == gw {
			continue
		}
		cand := make(net.IP, 4)
		binary.BigEndian.PutUint32(cand, host)
		if _, ok := used[cand.String()]; ok {
			continue
		}
		return cand, nil
	}
	return nil, fmt.Errorf("no free IPs in %s", ipNet.String())
}

func GuestCIDR(ip net.IP, ipNet *net.IPNet) string {
	ones, _ := ipNet.Mask.Size()
	return fmt.Sprintf("%s/%d", ip.String(), ones)
}

func ShortHex(id string, n int) string {
	s := strings.ReplaceAll(id, "-", "")
	if len(s) > n {
		s = s[:n]
	}
	return s
}

func BridgeName(networkID string) string {
	return "tn" + ShortHex(networkID, 12)
}

func VethHostName(instanceID string) string {
	return "th" + ShortHex(instanceID, 12)
}
