package crypto

import (
	"bytes"
	"testing"
)

func testKey(b byte) []byte {
	k := make([]byte, KeySize)
	for i := range k {
		k[i] = b
	}
	return k
}

func TestNewRejectsWrongKeySize(t *testing.T) {
	for _, n := range []int{0, 16, 31, 33, 64} {
		if _, err := New(make([]byte, n)); err == nil {
			t.Errorf("New with %d-byte key: expected error, got nil", n)
		}
	}
}

func TestSealOpenRoundTrip(t *testing.T) {
	c, err := New(testKey(0x01))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	cases := [][]byte{
		[]byte(""),
		[]byte("a"),
		[]byte("the quick brown fox"),
		bytes.Repeat([]byte{0xff}, 4096),
	}
	for _, pt := range cases {
		blob, err := c.Seal(pt)
		if err != nil {
			t.Fatalf("Seal: %v", err)
		}
		got, err := c.Open(blob)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if !bytes.Equal(got, pt) {
			t.Errorf("round-trip mismatch: got %q want %q", got, pt)
		}
	}
}

func TestSealProducesDistinctBlobs(t *testing.T) {
	c, _ := New(testKey(0x02))
	pt := []byte("same plaintext")
	a, _ := c.Seal(pt)
	b, _ := c.Seal(pt)
	if bytes.Equal(a, b) {
		t.Error("two Seals of the same plaintext produced identical blobs (nonce reuse)")
	}
}

func TestOpenRejectsTamper(t *testing.T) {
	c, _ := New(testKey(0x03))
	blob, _ := c.Seal([]byte("secret"))
	// Flip a bit in the ciphertext portion.
	blob[len(blob)-1] ^= 0x01
	if _, err := c.Open(blob); err == nil {
		t.Error("Open accepted a tampered blob")
	}
}

func TestOpenRejectsWrongKey(t *testing.T) {
	c1, _ := New(testKey(0x04))
	c2, _ := New(testKey(0x05))
	blob, _ := c1.Seal([]byte("secret"))
	if _, err := c2.Open(blob); err == nil {
		t.Error("Open accepted a blob sealed under a different key")
	}
}

func TestOpenRejectsShortBlob(t *testing.T) {
	c, _ := New(testKey(0x06))
	if _, err := c.Open([]byte{0x00, 0x01}); err == nil {
		t.Error("Open accepted a blob shorter than the nonce")
	}
}
