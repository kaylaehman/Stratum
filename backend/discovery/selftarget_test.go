package discovery

import (
	"context"
	"net"
	"testing"
)

func TestAnyIPMatches(t *testing.T) {
	a := []net.IP{net.ParseIP("10.0.0.5"), net.ParseIP("192.168.1.20")}
	b := []net.IP{net.ParseIP("172.16.0.1"), net.ParseIP("192.168.1.20")}
	if !anyIPMatches(a, b) {
		t.Error("expected a match on 192.168.1.20")
	}
	if anyIPMatches(a, []net.IP{net.ParseIP("8.8.8.8")}) {
		t.Error("did not expect a match against 8.8.8.8")
	}
	if anyIPMatches(nil, b) {
		t.Error("empty input should never match")
	}
}

func TestIsSelfTarget(t *testing.T) {
	cases := []struct {
		name string
		host string
		want bool
	}{
		{"localhost literal", "localhost", true},
		{"localhost mixed case + space", "  LocalHost ", true},
		{"ipv4 loopback", "127.0.0.1", true},
		{"ipv6 loopback", "::1", true},
		{"ipv6 loopback bracketed", "[::1]", true},
		{"empty", "", false},
		{"public ip is not self", "8.8.8.8", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isSelfTarget(context.Background(), c.host); got != c.want {
				t.Errorf("isSelfTarget(%q) = %v, want %v", c.host, got, c.want)
			}
		})
	}
}
