package updates

import "testing"

func TestCategory(t *testing.T) {
	tests := []struct {
		name   string
		reason string
		want   string
	}{
		{"empty -> other", "", CatOther},

		// Fixed reasons emitted by Classify.
		{"locally built", "no repo digest (locally-built or never pushed)", CatLocallyBuilt},
		{"daemon error", "local digest unavailable: cannot connect to docker daemon", CatDaemonError},
		{"empty digest", "registry returned empty digest", CatEmptyDigest},
		{"generic registry failure", "registry lookup failed: dial tcp: i/o timeout", CatRegistryUnreachable},

		// Auth substrings (checked before generic registry-unreachable).
		{"auth 401", "registry lookup failed: GET https://registry: 401 Unauthorized", CatAuth},
		{"auth 403", "registry lookup failed: 403 Forbidden", CatAuth},
		{"auth unauthorized word", "registry lookup failed: unauthorized: authentication required", CatAuth},
		{"auth denied word", "registry lookup failed: denied: requested access to the resource is denied", CatAuth},

		// Rate-limit substrings (checked before generic registry-unreachable).
		{"rate 429", "registry lookup failed: 429 Too Many Requests", CatRateLimited},
		{"rate toomanyrequests", "registry lookup failed: toomanyrequests: pull rate exceeded", CatRateLimited},
		{"rate limit phrase", "registry lookup failed: you have reached your pull rate limit", CatRateLimited},

		// Case-insensitivity.
		{"uppercase auth", "REGISTRY LOOKUP FAILED: 401 UNAUTHORIZED", CatAuth},
		{"mixed-case rate", "Registry Lookup Failed: TooManyRequests", CatRateLimited},

		// Unmatched reason falls through to other.
		{"unrecognized -> other", "some weird unexpected reason", CatOther},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Category(tt.reason); got != tt.want {
				t.Errorf("Category(%q) = %q, want %q", tt.reason, got, tt.want)
			}
		})
	}
}
