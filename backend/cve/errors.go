package cve

import "errors"

// ErrUnavailable is returned when the Trivy binary is not installed/configured.
var ErrUnavailable = errors.New("cve: scanner (trivy) not available")
