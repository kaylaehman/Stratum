package proxmox_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kaylaehman/stratum/backend/proxmox"
)

const (
	testTokenID = "root@pam!mytoken"
	testSecret  = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
)

// wantAuthHeader is the exact Authorization header value the client must send.
const wantAuthHeader = "PVEAPIToken=" + testTokenID + "=" + testSecret

// TestVersion_200 verifies a successful Proxmox version probe.
func TestVersion_200(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Assert the correct Authorization header was sent.
		got := r.Header.Get("Authorization")
		if got != wantAuthHeader {
			t.Errorf("Authorization header = %q; want %q", got, wantAuthHeader)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"version":"8.2.4","release":"4","repoid":"abc123"}}`))
	}))
	defer srv.Close()

	c := proxmox.New(srv.URL, testTokenID, testSecret, false)
	version, status, err := c.Version(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != proxmox.AuthConfirmed {
		t.Errorf("status = %q; want %q", status, proxmox.AuthConfirmed)
	}
	if version != "8.2.4" {
		t.Errorf("version = %q; want %q", version, "8.2.4")
	}
}

// TestVersion_401 verifies that a 401 response is treated as AuthUnauthed with
// no error — the endpoint IS Proxmox, the token is just wrong/missing.
func TestVersion_401(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := proxmox.New(srv.URL, testTokenID, testSecret, false)
	version, status, err := c.Version(context.Background())

	if err != nil {
		t.Fatalf("expected nil error on 401, got: %v", err)
	}
	if status != proxmox.AuthUnauthed {
		t.Errorf("status = %q; want %q", status, proxmox.AuthUnauthed)
	}
	if version != "" {
		t.Errorf("version = %q; want empty string", version)
	}
}

// TestVersion_500 verifies that an unexpected HTTP status produces an error.
func TestVersion_500(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := proxmox.New(srv.URL, testTokenID, testSecret, false)
	_, _, err := c.Version(context.Background())

	if err == nil {
		t.Fatal("expected an error for HTTP 500, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error message %q does not mention status 500", err.Error())
	}
}

// TestVersion_TLS verifies the insecureSkipVerify toggle using httptest.NewTLSServer
// (which always presents a self-signed certificate).
func TestVersion_TLS(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"version":"8.1.0","release":"3","repoid":"xyz"}}`))
	}))
	defer srv.Close()

	// Subtests are sequential (no t.Parallel) so the deferred srv.Close() does
	// not fire until both subtests complete.

	// With insecureSkipVerify=false the TLS handshake must fail (cert not trusted).
	t.Run("strict_tls_fails", func(t *testing.T) {
		c := proxmox.New(srv.URL, testTokenID, testSecret, false)
		_, _, err := c.Version(context.Background())
		if err == nil {
			t.Fatal("expected TLS error with insecureSkipVerify=false, got nil")
		}
	})

	// With insecureSkipVerify=true the self-signed cert is accepted and the
	// probe succeeds.
	t.Run("skip_verify_succeeds", func(t *testing.T) {
		c := proxmox.New(srv.URL, testTokenID, testSecret, true)
		version, status, err := c.Version(context.Background())
		if err != nil {
			t.Fatalf("unexpected error with insecureSkipVerify=true: %v", err)
		}
		if status != proxmox.AuthConfirmed {
			t.Errorf("status = %q; want %q", status, proxmox.AuthConfirmed)
		}
		if version != "8.1.0" {
			t.Errorf("version = %q; want %q", version, "8.1.0")
		}
	})
}
