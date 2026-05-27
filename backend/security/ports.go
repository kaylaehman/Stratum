package security

import "github.com/kaylaehman/stratum/backend/docker"

// Interface classes for a published port's host bind address.
const (
	IfaceAll      = "all"      // 0.0.0.0 / :: — reachable on every interface
	IfaceLoopback = "loopback" // 127.0.0.0/8 / ::1 — local only
	IfaceExternal = "external" // a specific non-loopback IP (e.g. LAN) — reachable
)

// PortExposure is a published container port classified by host bind interface.
type PortExposure struct {
	HostIP         string `json:"host_ip"`
	HostPort       int    `json:"host_port"`
	ContainerPort  int    `json:"container_port"`
	Protocol       string `json:"protocol"`
	InterfaceClass string `json:"interface_class"`
}

// classifyInterface buckets a host bind IP. Anything that isn't all-interfaces
// or loopback is "external" (a specific reachable IP) — flagged, not neutral.
func classifyInterface(hostIP string) string {
	switch {
	case hostIP == "0.0.0.0" || hostIP == "::" || hostIP == "":
		return IfaceAll
	case hostIP == "127.0.0.1" || hostIP == "::1" || hasPrefix(hostIP, "127."):
		return IfaceLoopback
	default:
		return IfaceExternal
	}
}

func hasPrefix(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }

// classifyPorts converts inspect port bindings into classified exposures.
func classifyPorts(ports []docker.PortBinding) []PortExposure {
	out := make([]PortExposure, 0, len(ports))
	for _, p := range ports {
		out = append(out, PortExposure{
			HostIP:         p.HostIP,
			HostPort:       p.HostPort,
			ContainerPort:  p.ContainerPort,
			Protocol:       p.Protocol,
			InterfaceClass: classifyInterface(p.HostIP),
		})
	}
	return out
}
