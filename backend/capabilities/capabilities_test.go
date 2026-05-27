package capabilities

import (
	"errors"
	"testing"
)

func TestParseEmpty(t *testing.T) {
	for _, in := range [][]byte{nil, {}, []byte("{}")} {
		s, err := Parse(in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", in, err)
		}
		if s != (Set{}) {
			t.Errorf("Parse(%q) = %+v, want zero Set", in, s)
		}
	}
}

func TestParseFull(t *testing.T) {
	s, err := Parse([]byte(`{"proxmox":true,"docker":true,"agent":false,"systemd":true,"cron":true}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !s.Proxmox || !s.Docker || s.Agent || !s.Systemd || !s.Cron {
		t.Errorf("Parse decoded wrong: %+v", s)
	}
}

func TestParseInvalid(t *testing.T) {
	if _, err := Parse([]byte("not json")); err == nil {
		t.Error("Parse accepted invalid JSON")
	}
}

func TestRequirePresent(t *testing.T) {
	s := Set{Docker: true}
	if err := Require(s, Docker); err != nil {
		t.Errorf("Require(docker) on docker-capable set: %v", err)
	}
}

func TestRequireAbsent(t *testing.T) {
	s := Set{Docker: true}
	err := Require(s, Proxmox)
	if err == nil {
		t.Fatal("Require(proxmox) on non-proxmox set: expected error")
	}
	var capErr *ErrCapabilityUnavailable
	if !errors.As(err, &capErr) {
		t.Fatalf("expected *ErrCapabilityUnavailable, got %T", err)
	}
	if capErr.Capability != Proxmox {
		t.Errorf("error names %q, want proxmox", capErr.Capability)
	}
}

func TestHasUnknown(t *testing.T) {
	if (Set{Proxmox: true, Docker: true}).Has(Capability("bogus")) {
		t.Error("Has returned true for unknown capability")
	}
}
