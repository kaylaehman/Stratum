package docker

import (
	"path"
	"strings"
)

// Exposure is the result of checking whether a host path is mounted into a
// container. Shared by SP5 (diagnostic) and SP7 (bind-mount tracer).
type Exposure struct {
	Exposed       bool   `json:"exposed"`
	ContainerPath string `json:"container_path,omitempty"`
	RW            bool   `json:"rw"`
	ViaSource     string `json:"via_source,omitempty"`      // the matching mount's host source
	ViaDest       string `json:"via_destination,omitempty"` // the mount's container destination
	IsNamedVolume bool   `json:"is_named_volume"`
	VolumeName    string `json:"volume_name,omitempty"`
}

// Forward reports whether hostPath is exposed into the container by any of its
// mounts, and the corresponding in-container path. Matching is segment-aware
// (so /data does not match /data-archive); the most specific (longest source)
// mount wins.
func Forward(hostPath string, mounts []Mount) Exposure {
	hostPath = path.Clean(hostPath)
	best := Exposure{}
	bestLen := -1
	for _, m := range mounts {
		src := path.Clean(m.Source)
		if src == "" || src == "." {
			continue
		}
		if hostPath != src && !strings.HasPrefix(hostPath, src+"/") {
			continue
		}
		if len(src) <= bestLen {
			continue
		}
		bestLen = len(src)
		rel := strings.TrimPrefix(hostPath, src)
		best = Exposure{
			Exposed:       true,
			ContainerPath: path.Join(m.Destination, rel),
			RW:            m.RW,
			ViaSource:     m.Source,
			ViaDest:       m.Destination,
		}
		if m.Type == "volume" {
			best.IsNamedVolume = true
			best.VolumeName = volumeName(src)
		}
	}
	return best
}

// volumeName extracts <name> from /var/lib/docker/volumes/<name>/_data.
func volumeName(source string) string {
	const prefix = "/var/lib/docker/volumes/"
	const suffix = "/_data"
	if strings.HasPrefix(source, prefix) {
		rest := strings.TrimPrefix(source, prefix)
		rest = strings.TrimSuffix(rest, suffix)
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			return rest[:i]
		}
		return rest
	}
	return ""
}
