package backup

import "errors"

// ErrInvalidInput is returned for an invalid volume name or destination dir.
var ErrInvalidInput = errors.New("backup: invalid volume or destination")
