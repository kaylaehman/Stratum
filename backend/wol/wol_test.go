package wol

import (
	"bytes"
	"net"
	"testing"
	"time"
)

func TestMagicPacket(t *testing.T) {
	pkt, err := MagicPacket("01:02:03:04:05:06")
	if err != nil {
		t.Fatalf("MagicPacket: %v", err)
	}
	if len(pkt) != 102 {
		t.Fatalf("packet len = %d, want 102", len(pkt))
	}
	// First 6 bytes are 0xFF.
	for i := 0; i < 6; i++ {
		if pkt[i] != 0xFF {
			t.Fatalf("byte %d = %#x, want 0xFF", i, pkt[i])
		}
	}
	mac := []byte{1, 2, 3, 4, 5, 6}
	// 16 repetitions of the MAC follow.
	for rep := 0; rep < 16; rep++ {
		off := 6 + rep*6
		if !bytes.Equal(pkt[off:off+6], mac) {
			t.Fatalf("repetition %d = %v, want %v", rep, pkt[off:off+6], mac)
		}
	}
}

func TestMagicPacketInvalidMAC(t *testing.T) {
	if _, err := MagicPacket("not-a-mac"); err == nil {
		t.Error("expected error for invalid MAC")
	}
}

func TestSendDeliversPacket(t *testing.T) {
	// Listen on a loopback UDP port and verify Send delivers the magic packet.
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	port := conn.LocalAddr().(*net.UDPAddr).Port

	if err := Send("aa:bb:cc:dd:ee:ff", "127.0.0.1", port); err != nil {
		t.Fatalf("Send: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 128)
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want, _ := MagicPacket("aa:bb:cc:dd:ee:ff")
	if !bytes.Equal(buf[:n], want) {
		t.Errorf("received %d bytes, not the expected magic packet", n)
	}
}
