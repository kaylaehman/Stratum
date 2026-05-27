package nodes_test

import (
	"bytes"
	"testing"

	"github.com/kaylaehman/stratum/backend/crypto"
	"github.com/kaylaehman/stratum/backend/nodes"
)

func testCipher(t *testing.T, b byte) *crypto.Cipher {
	t.Helper()
	key := make([]byte, crypto.KeySize)
	for i := range key {
		key[i] = b
	}
	c, err := crypto.New(key)
	if err != nil {
		t.Fatalf("crypto.New: %v", err)
	}
	return c
}

func TestCredentialsSealOpenRoundTrip(t *testing.T) {
	c := testCipher(t, 0x01)
	creds := nodes.NodeCredentials{
		Method:        nodes.MethodSSHKey,
		SSHUser:       "kayla",
		SSHPrivateKey: "-----BEGIN OPENSSH PRIVATE KEY-----\nsecret\n-----END-----",
		SSHPassphrase: "hunter2",
	}
	blob, err := creds.Seal(c)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	got, err := nodes.OpenCredentials(c, blob)
	if err != nil {
		t.Fatalf("OpenCredentials: %v", err)
	}
	if got != creds {
		t.Errorf("round-trip mismatch: %+v != %+v", got, creds)
	}
}

func TestSealedBlobContainsNoPlaintextSecret(t *testing.T) {
	c := testCipher(t, 0x02)
	creds := nodes.NodeCredentials{Method: nodes.MethodSSHPassword, SSHUser: "kayla", SSHPassword: "SuperSecretP@ss"}
	blob, err := creds.Seal(c)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(blob, []byte("SuperSecretP@ss")) {
		t.Error("sealed blob leaks the plaintext password")
	}
	if bytes.Contains(blob, []byte("kayla")) {
		t.Error("sealed blob leaks the plaintext username")
	}
}

func TestOpenWithWrongKeyFails(t *testing.T) {
	blob, err := nodes.NodeCredentials{Method: nodes.MethodSSHPassword, SSHPassword: "x"}.Seal(testCipher(t, 0x03))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nodes.OpenCredentials(testCipher(t, 0x04), blob); err == nil {
		t.Error("OpenCredentials accepted a blob sealed under a different key")
	}
}
