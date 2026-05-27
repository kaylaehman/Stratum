// Package wol implements Wake-on-LAN (Feature 6): build and send the magic
// packet that powers on a node by MAC address. The packet builder is pure and
// unit-tested; Send writes a UDP datagram (typically to a broadcast address).
package wol

import (
	"fmt"
	"net"
	"time"
)

// MagicPacket builds the 102-byte WOL magic packet for a MAC: 6 bytes of 0xFF
// followed by the 6-byte MAC repeated 16 times. The MAC may be in any form
// net.ParseMAC accepts (e.g. "AA:BB:CC:DD:EE:FF").
func MagicPacket(mac string) ([]byte, error) {
	hw, err := net.ParseMAC(mac)
	if err != nil {
		return nil, fmt.Errorf("wol: invalid MAC %q: %w", mac, err)
	}
	if len(hw) != 6 {
		return nil, fmt.Errorf("wol: MAC must be 6 bytes (EUI-48), got %d", len(hw))
	}
	pkt := make([]byte, 0, 6+16*6)
	for i := 0; i < 6; i++ {
		pkt = append(pkt, 0xFF)
	}
	for i := 0; i < 16; i++ {
		pkt = append(pkt, hw...)
	}
	return pkt, nil
}

// Send builds and sends the magic packet to broadcast:port over UDP. An empty
// broadcast defaults to the limited broadcast address; port 0 defaults to 9.
func Send(mac, broadcast string, port int) error {
	pkt, err := MagicPacket(mac)
	if err != nil {
		return err
	}
	if broadcast == "" {
		broadcast = "255.255.255.255"
	}
	if port == 0 {
		port = 9
	}
	addr := net.JoinHostPort(broadcast, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("udp", addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("wol: dial %s: %w", addr, err)
	}
	defer conn.Close()
	if _, err := conn.Write(pkt); err != nil {
		return fmt.Errorf("wol: send: %w", err)
	}
	return nil
}
