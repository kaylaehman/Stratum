package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("metrics"))
	})
}

func TestMetricsGuard_TokenMode(t *testing.T) {
	h := metricsGuard("s3cret", okHandler())

	cases := []struct {
		name       string
		authHeader string
		remote     string
		want       int
	}{
		{"correct token from remote", "Bearer s3cret", "10.0.0.9:5000", http.StatusOK},
		{"wrong token", "Bearer nope", "10.0.0.9:5000", http.StatusUnauthorized},
		{"missing header", "", "127.0.0.1:5000", http.StatusUnauthorized},
		{"non-bearer scheme", "Basic s3cret", "127.0.0.1:5000", http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			req.RemoteAddr = tc.remote
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("code = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestMetricsGuard_LoopbackFallback(t *testing.T) {
	h := metricsGuard("", okHandler()) // no token → loopback-only

	loop := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	loop.RemoteAddr = "127.0.0.1:5000"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, loop)
	if rec.Code != http.StatusOK {
		t.Errorf("loopback: code = %d, want 200", rec.Code)
	}

	remote := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	remote.RemoteAddr = "192.168.1.50:5000"
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, remote)
	if rec2.Code != http.StatusForbidden {
		t.Errorf("remote without token: code = %d, want 403", rec2.Code)
	}
}
