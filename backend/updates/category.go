package updates

import "strings"

// Category buckets for "unknown" image-update results. The UI uses these to
// distinguish a benign locally-built image ("nothing to check") from an
// actionable failure (auth, rate limit, unreachable registry, daemon error).
const (
	CatLocallyBuilt        = "locally_built"        // "no repo digest (locally-built…)"
	CatRegistryUnreachable = "registry_unreachable" // generic "registry lookup failed: …"
	CatAuth                = "auth"                 // 401/403/unauthorized/denied in the reason
	CatRateLimited         = "rate_limited"         // 429/toomanyrequests/rate limit in the reason
	CatDaemonError         = "daemon_error"         // "local digest unavailable: …"
	CatEmptyDigest         = "empty_digest"         // "registry returned empty digest"
	CatOther               = "other"
)

// Category classifies an unknownReason string (as produced by Classify) into one
// of the Cat* buckets via case-insensitive substring matching.
//
// Order matters: auth and rate-limit substrings are checked BEFORE the generic
// "registry lookup failed" prefix, because a failed registry lookup whose error
// text mentions 401/429 should be attributed to the more specific cause. An
// empty reason maps to CatOther.
func Category(unknownReason string) string {
	if unknownReason == "" {
		return CatOther
	}
	r := strings.ToLower(unknownReason)

	// Most-specific registry failures first.
	if strings.Contains(r, "401") ||
		strings.Contains(r, "403") ||
		strings.Contains(r, "unauthorized") ||
		strings.Contains(r, "denied") {
		return CatAuth
	}
	if strings.Contains(r, "429") ||
		strings.Contains(r, "toomanyrequests") ||
		strings.Contains(r, "rate limit") {
		return CatRateLimited
	}

	// Fixed-reason buckets.
	switch {
	case strings.Contains(r, "no repo digest"):
		return CatLocallyBuilt
	case strings.Contains(r, "local digest unavailable"):
		return CatDaemonError
	case strings.Contains(r, "registry returned empty digest"):
		return CatEmptyDigest
	case strings.Contains(r, "registry lookup failed"):
		return CatRegistryUnreachable
	}

	return CatOther
}
