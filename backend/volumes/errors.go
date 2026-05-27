package volumes

import "errors"

// ErrVolumeInUse is returned when a remove is attempted on a volume the mount
// index still shows as attached to a container.
var ErrVolumeInUse = errors.New("volumes: volume is in use")
