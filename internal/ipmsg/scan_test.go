package ipmsg

import (
	"net"
	"testing"
)

func TestParseScanTargetRange(t *testing.T) {
	cases := []struct {
		target string
		count  int
		first  string
		last   string
	}{
		{"172.17.32.1-172.17.32.254", 254, "172.17.32.1", "172.17.32.254"},
		{"172.17.32.1-254", 254, "172.17.32.1", "172.17.32.254"},
		{"10.0.0.254-10.0.1.1", 4, "10.0.0.254", "10.0.1.1"},
		{"10.1.2.3-10.1.2.3", 1, "10.1.2.3", "10.1.2.3"},
	}
	for _, tc := range cases {
		ips, err := parseScanTarget(tc.target, nil)
		if err != nil {
			t.Fatalf("%s: %v", tc.target, err)
		}
		if len(ips) != tc.count {
			t.Fatalf("%s: count = %d, want %d (first=%v)", tc.target, len(ips), tc.count, ips[0])
		}
		if got := ips[0].String(); got != tc.first {
			t.Errorf("%s: first = %s, want %s", tc.target, got, tc.first)
		}
		if got := ips[len(ips)-1].String(); got != tc.last {
			t.Errorf("%s: last = %s, want %s", tc.target, got, tc.last)
		}
	}
}

func TestParseScanTargetRangeRejectsReversed(t *testing.T) {
	if _, err := parseScanTarget("10.0.0.9-10.0.0.1", nil); err == nil {
		t.Fatal("expected an error for a reversed range")
	}
}

func TestParseScanTargetCIDR(t *testing.T) {
	ips, err := parseScanTarget("10.1.2.0/30", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) != 4 || ips[0].String() != "10.1.2.0" || ips[3].String() != "10.1.2.3" {
		t.Fatalf("got %d addresses: %v", len(ips), ips)
	}
	if _, err := parseScanTarget("fd00::/126", nil); err == nil {
		t.Fatal("expected IPv6 CIDR to be rejected")
	}
}

func TestParseScanTargetOtherForms(t *testing.T) {
	ips, err := parseScanTarget("192.168.1.", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) != 254 || ips[0].String() != "192.168.1.1" || ips[253].String() != "192.168.1.254" {
		t.Fatalf("dot form: got %d addresses", len(ips))
	}

	ips, err = parseScanTarget("192.168.1.7", nil)
	if err != nil || len(ips) != 1 || !ips[0].Equal(net.ParseIP("192.168.1.7")) {
		t.Fatalf("single address: %v, %v", ips, err)
	}

	ips, err = parseScanTarget("", map[string]bool{"10.5.0.9": true})
	if err != nil || len(ips) != 254 || ips[0].String() != "10.5.0.1" {
		t.Fatalf("empty target: %v, %v", len(ips), err)
	}

	if _, err := parseScanTarget("not-an-ip", nil); err == nil {
		t.Fatal("expected an error for an invalid target")
	}
}
