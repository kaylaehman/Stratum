package security

import (
	"context"
	"regexp"
	"strconv"
	"strings"
)

// Listener is one listening host socket.
type Listener struct {
	Protocol string `json:"protocol"` // tcp | udp
	Address  string `json:"address"`  // bind address, e.g. 0.0.0.0, 127.0.0.1, ::, a LAN IP
	Port     int    `json:"port"`
	Process  string `json:"process"` // process name if available, else ""
}

// ListenerResult is the outcome. Available is false when ss is missing/errored.
type ListenerResult struct {
	Available bool       `json:"available"`
	Listeners []Listener `json:"listeners,omitempty"`
}

// ExecFunc runs a command on a node and returns stdout. Injected for transport-agnosticism + testing.
type ExecFunc func(ctx context.Context, nodeID, cmd string, args ...string) (string, error)

// GetListeners runs `ss -tulnpH` via exec and parses the result.
// A non-nil exec error (missing ss / non-zero) yields Available:false (not an error).
func GetListeners(ctx context.Context, nodeID string, exec ExecFunc) ListenerResult {
	out, err := exec(ctx, nodeID, "ss", "-tulnpH")
	if err != nil {
		return ListenerResult{Available: false}
	}
	return ListenerResult{
		Available: true,
		Listeners: parseSS(out),
	}
}

// reProcess matches the users:(("name",...)) field in ss output.
var reProcess = regexp.MustCompile(`users:\(\("([^"]+)"`)

// parseSS parses `ss -tulnpH` output into Listeners.
// Robust to column-width variation across iproute2 versions.
func parseSS(out string) []Listener {
	var listeners []Listener
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		l, ok := parseLine(line)
		if ok {
			listeners = append(listeners, l)
		}
	}
	return listeners
}

// parseLine parses a single ss output line into a Listener.
func parseLine(line string) (Listener, bool) {
	// ss -tulnpH columns (space-separated, variable width):
	//   Netid  State  Recv-Q  Send-Q  Local-Addr:Port  Peer-Addr:Port  [Process]
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return Listener{}, false
	}

	netid := strings.ToLower(fields[0])
	if netid != "tcp" && netid != "udp" {
		return Listener{}, false
	}

	// Local address is always fields[4].
	localField := fields[4]
	addr, port, ok := splitAddrPort(localField)
	if !ok {
		return Listener{}, false
	}

	proc := ""
	// Process field may be in fields[6] or later (it starts with "users:(")
	for i := 5; i < len(fields); i++ {
		if strings.HasPrefix(fields[i], "users:(") {
			m := reProcess.FindStringSubmatch(fields[i])
			if len(m) == 2 {
				proc = m[1]
			}
			break
		}
	}

	return Listener{
		Protocol: netid,
		Address:  addr,
		Port:     port,
		Process:  proc,
	}, true
}

// splitAddrPort splits a Local-Address:Port token from ss output.
// Handles: 0.0.0.0:22, *:8080, 127.0.0.1:5432, [::]:80, [::1]:443, :::80, ::1:443.
func splitAddrPort(s string) (addr string, port int, ok bool) {
	// IPv6 with brackets: [::]:80 or [::1]:443
	if strings.HasPrefix(s, "[") {
		closeBracket := strings.LastIndex(s, "]")
		if closeBracket == -1 {
			return "", 0, false
		}
		rawAddr := s[1:closeBracket]
		rest := s[closeBracket+1:]
		if !strings.HasPrefix(rest, ":") {
			return "", 0, false
		}
		portStr := rest[1:]
		p, err := strconv.Atoi(portStr)
		if err != nil {
			return "", 0, false
		}
		return normalizeAddr(rawAddr), p, true
	}

	// Port is always after the LAST colon.
	lastColon := strings.LastIndex(s, ":")
	if lastColon == -1 {
		return "", 0, false
	}
	portStr := s[lastColon+1:]
	rawAddr := s[:lastColon]

	p, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, false
	}

	return normalizeAddr(rawAddr), p, true
}

// normalizeAddr converts * -> 0.0.0.0 and strips any remaining brackets.
func normalizeAddr(addr string) string {
	addr = strings.Trim(addr, "[]")
	if addr == "*" {
		return "0.0.0.0"
	}
	return addr
}
