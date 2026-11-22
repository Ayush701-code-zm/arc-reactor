package network

import (
	"testing"
)

func TestParseCIDRAndGateway(t *testing.T) {
	ipNet, err := ParseCIDR("10.10.0.0/24")
	if err != nil {
		t.Fatal(err)
	}
	gw, err := GatewayIP(ipNet)
	if err != nil {
		t.Fatal(err)
	}
	if gw.String() != "10.10.0.1" {
		t.Fatalf("gateway=%s", gw)
	}
}

func TestAllocateGuestIPSkipsUsed(t *testing.T) {
	ipNet, err := ParseCIDR("10.10.0.0/29")
	if err != nil {
		t.Fatal(err)
	}
	gw, err := GatewayIP(ipNet)
	if err != nil {
		t.Fatal(err)
	}
	used := map[string]struct{}{
		"10.10.0.1": {},
		"10.10.0.2": {},
	}
	ip, err := AllocateGuestIP(ipNet, gw, used)
	if err != nil {
		t.Fatal(err)
	}
	if ip.String() != "10.10.0.3" {
		t.Fatalf("got %s", ip)
	}
	cidr := GuestCIDR(ip, ipNet)
	if cidr != "10.10.0.3/29" {
		t.Fatalf("cidr=%s", cidr)
	}
}

func TestAllocateGuestIPExhausted(t *testing.T) {
	ipNet, _ := ParseCIDR("10.10.0.0/30")
	gw, _ := GatewayIP(ipNet)
	used := map[string]struct{}{"10.10.0.1": {}, "10.10.0.2": {}}
	if _, err := AllocateGuestIP(ipNet, gw, used); err == nil {
		t.Fatal("expected exhaustion")
	}
}

func TestNames(t *testing.T) {
	id := "aabbccdd-eeff-1122-3344-556677889900"
	if BridgeName(id) != "tnaabbccddeeff" {
		t.Fatalf("bridge=%s", BridgeName(id))
	}
	if VethHostName(id) != "thaabbccddeeff" {
		t.Fatalf("veth=%s", VethHostName(id))
	}
}
