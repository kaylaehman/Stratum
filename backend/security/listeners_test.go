package security

import (
	"context"
	"errors"
	"testing"
)

// sampleSSOutput is a realistic multi-line `ss -tulnpH` sample.
const sampleSSOutput = `tcp   LISTEN 0      128          0.0.0.0:22        0.0.0.0:*    users:(("sshd",pid=712,fd=3))
tcp   LISTEN 0      128        127.0.0.1:5432      0.0.0.0:*    users:(("postgres",pid=1024,fd=5))
tcp   LISTEN 0      128              *:8080          *:*          users:(("docker-proxy",pid=2048,fd=4))
tcp   LISTEN 0      128           [::]:80           [::]:*       users:(("nginx",pid=3001,fd=6))
udp   UNCONN 0      0            0.0.0.0:53        0.0.0.0:*    users:(("dnsmasq",pid=890,fd=7))
tcp   LISTEN 0      128          0.0.0.0:443       0.0.0.0:*
`

// expectedListeners maps test indices to expected values.
var expectedListeners = []Listener{
	{Protocol: "tcp", Address: "0.0.0.0", Port: 22, Process: "sshd"},
	{Protocol: "tcp", Address: "127.0.0.1", Port: 5432, Process: "postgres"},
	{Protocol: "tcp", Address: "0.0.0.0", Port: 8080, Process: "docker-proxy"},
	{Protocol: "tcp", Address: "::", Port: 80, Process: "nginx"},
	{Protocol: "udp", Address: "0.0.0.0", Port: 53, Process: "dnsmasq"},
	{Protocol: "tcp", Address: "0.0.0.0", Port: 443, Process: ""},
}

func TestParseSS(t *testing.T) {
	got := parseSS(sampleSSOutput)
	if len(got) != len(expectedListeners) {
		t.Fatalf("parseSS: got %d listeners, want %d\ngot: %+v", len(got), len(expectedListeners), got)
	}
	for i, want := range expectedListeners {
		g := got[i]
		if g.Protocol != want.Protocol {
			t.Errorf("line %d: Protocol got %q want %q", i, g.Protocol, want.Protocol)
		}
		if g.Address != want.Address {
			t.Errorf("line %d: Address got %q want %q", i, g.Address, want.Address)
		}
		if g.Port != want.Port {
			t.Errorf("line %d: Port got %d want %d", i, g.Port, want.Port)
		}
		if g.Process != want.Process {
			t.Errorf("line %d: Process got %q want %q", i, g.Process, want.Process)
		}
	}
}

func TestGetListeners_Success(t *testing.T) {
	fakeExec := func(_ context.Context, _ string, cmd string, args ...string) (string, error) {
		if cmd != "ss" || len(args) < 1 || args[0] != "-tulnpH" {
			t.Errorf("unexpected exec call: cmd=%q args=%v", cmd, args)
		}
		return sampleSSOutput, nil
	}

	result := GetListeners(context.Background(), "node-1", fakeExec)
	if !result.Available {
		t.Fatal("GetListeners: expected Available=true")
	}
	if len(result.Listeners) != len(expectedListeners) {
		t.Fatalf("GetListeners: got %d listeners, want %d", len(result.Listeners), len(expectedListeners))
	}
}

func TestGetListeners_ExecError(t *testing.T) {
	fakeExec := func(_ context.Context, _ string, _ string, _ ...string) (string, error) {
		return "", errors.New("ss: command not found")
	}

	result := GetListeners(context.Background(), "node-1", fakeExec)
	if result.Available {
		t.Fatal("GetListeners: expected Available=false on exec error")
	}
	if len(result.Listeners) != 0 {
		t.Fatalf("GetListeners: expected no listeners on error, got %d", len(result.Listeners))
	}
}
