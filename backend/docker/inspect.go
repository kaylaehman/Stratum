package docker

// inspect.go — container inspect, top, and file-copy operations.
//
// RepoDigests note: InspectResponse does not carry image repo-digests directly;
// those live on the image object returned by ImageInspect. InspectInfo.RepoDigests
// is always left empty here; callers that need it should follow up with an
// ImageInspect call using InspectInfo.ImageID.

import (
	"archive/tar"
	"context"
	"errors"
	"io"
	"strconv"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
)

// Mount is a faithful subset of a container mount point.
type Mount struct {
	Type        string `json:"type"`        // bind | volume | tmpfs
	Name        string `json:"name"`        // Docker volume name (type=volume only); canonical volume identity
	Source      string `json:"source"`      // host path (bind) or driver _data path (volume)
	Destination string `json:"destination"` // path inside container
	Mode        string `json:"mode"`
	RW          bool   `json:"rw"`
}

// InspectInfo is a faithful subset of container.InspectResponse, broadened for
// SP5 (Mounts), SP8 (Privileged/Caps/namespaces), and Update (RepoDigests).
type InspectInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`         // leading "/" stripped
	Image       string   `json:"image"`        // Config.Image
	ImageID     string   `json:"image_id"`     // .Image (the sha256 image id)
	State       string   `json:"state"`        // .State.Status (running|exited|...)
	ConfigUser  string   `json:"config_user"`  // .Config.User (reflects runtime --user merged over image USER)
	Tty         bool     `json:"tty"`          // .Config.Tty (drives log-stream demux)
	Mounts      []Mount  `json:"mounts"`
	Privileged  bool     `json:"privileged"`   // .HostConfig.Privileged
	CapAdd      []string `json:"cap_add"`      // .HostConfig.CapAdd
	CapDrop     []string `json:"cap_drop"`     // .HostConfig.CapDrop
	PidMode     string   `json:"pid_mode"`     // .HostConfig.PidMode
	NetworkMode string   `json:"network_mode"` // .HostConfig.NetworkMode
	SecurityOpt []string `json:"security_opt"` // .HostConfig.SecurityOpt (seccomp/apparmor=unconfined)
	Devices     []string `json:"devices"`      // .HostConfig.Devices as "host:container"
	UsernsMode  string   `json:"userns_mode"`  // .HostConfig.UsernsMode
	Ports       []PortBinding `json:"ports"`   // published ports from NetworkSettings.Ports
	RepoDigests []string `json:"repo_digests"` // always empty here; fill via ImageInspect(ImageID)
}

// PortBinding is one published container port -> host binding.
type PortBinding struct {
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"` // tcp | udp
	HostIP        string `json:"host_ip"`  // 0.0.0.0 | 127.0.0.1 | :: | specific
	HostPort      int    `json:"host_port"`
}

// mapMounts converts []container.MountPoint to []Mount.
func mapMounts(mps []container.MountPoint) []Mount {
	if len(mps) == 0 {
		return []Mount{}
	}
	out := make([]Mount, len(mps))
	for i, mp := range mps {
		out[i] = Mount{
			Type:        string(mp.Type),
			Name:        mp.Name,
			Source:      mp.Source,
			Destination: mp.Destination,
			Mode:        mp.Mode,
			RW:          mp.RW,
		}
	}
	return out
}

// mapInspect converts container.InspectResponse to InspectInfo.
func mapInspect(r container.InspectResponse) InspectInfo {
	var (
		state       string
		configUser  string
		configImage string
		tty         bool
		capAdd      []string
		capDrop     []string
		pidMode     string
		networkMode string
		securityOpt []string
		devices     []string
		usernsMode  string
		privileged  bool
	)

	if r.State != nil {
		state = r.State.Status
	}
	if r.Config != nil {
		configUser = r.Config.User
		configImage = r.Config.Image
		tty = r.Config.Tty
	}
	if r.HostConfig != nil {
		privileged = r.HostConfig.Privileged
		capAdd = []string(r.HostConfig.CapAdd)
		capDrop = []string(r.HostConfig.CapDrop)
		pidMode = string(r.HostConfig.PidMode)
		networkMode = string(r.HostConfig.NetworkMode)
		securityOpt = r.HostConfig.SecurityOpt
		usernsMode = string(r.HostConfig.UsernsMode)
		for _, d := range r.HostConfig.Devices {
			devices = append(devices, d.PathOnHost+":"+d.PathInContainer)
		}
	}
	if capAdd == nil {
		capAdd = []string{}
	}
	if capDrop == nil {
		capDrop = []string{}
	}
	if securityOpt == nil {
		securityOpt = []string{}
	}
	if devices == nil {
		devices = []string{}
	}

	return InspectInfo{
		ID:          r.ID,
		Name:        strings.TrimPrefix(r.Name, "/"),
		Image:       configImage,
		ImageID:     r.Image,
		State:       state,
		ConfigUser:  configUser,
		Tty:         tty,
		Mounts:      mapMounts(r.Mounts),
		Privileged:  privileged,
		CapAdd:      capAdd,
		CapDrop:     capDrop,
		PidMode:     pidMode,
		NetworkMode: networkMode,
		SecurityOpt: securityOpt,
		Devices:     devices,
		UsernsMode:  usernsMode,
		Ports:       mapPorts(r),
		RepoDigests: []string{},
	}
}

// mapPorts extracts published port bindings from NetworkSettings.Ports, skipping
// EXPOSE-only ports (nil/empty binding slice). The map key is "port/proto".
func mapPorts(r container.InspectResponse) []PortBinding {
	if r.NetworkSettings == nil {
		return []PortBinding{}
	}
	out := []PortBinding{}
	for portProto, bindings := range r.NetworkSettings.Ports {
		if len(bindings) == 0 {
			continue // EXPOSE-only, not published
		}
		cport, proto := splitPortProto(string(portProto))
		for _, b := range bindings {
			hp, _ := strconv.Atoi(b.HostPort)
			out = append(out, PortBinding{
				ContainerPort: cport,
				Protocol:      proto,
				HostIP:        b.HostIP,
				HostPort:      hp,
			})
		}
	}
	return out
}

// splitPortProto parses "80/tcp" into (80, "tcp").
func splitPortProto(s string) (int, string) {
	port, proto, found := strings.Cut(s, "/")
	if !found {
		proto = "tcp"
	}
	p, _ := strconv.Atoi(port)
	return p, proto
}

// Inspect returns a subset of container inspect data for the given container ID
// or name. The leading "/" is stripped from Name. RepoDigests is always empty;
// callers needing it should call ImageInspect with the returned ImageID.
func (c *Client) Inspect(ctx context.Context, id string) (InspectInfo, error) {
	r, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return InspectInfo{}, err
	}
	return mapInspect(r), nil
}

// ErrFileNotFoundInContainer is returned by CopyFromContainer when the
// requested path does not exist in the container (e.g. a distroless image).
var ErrFileNotFoundInContainer = errors.New("docker: file not found in container")

// ErrFileTooLargeInContainer is returned when a copied file exceeds the read
// cap, so callers never parse a silently-truncated file (e.g. /etc/passwd).
var ErrFileTooLargeInContainer = errors.New("docker: file too large to copy from container")

// mapCopyErr translates a Docker SDK copy error to ErrFileNotFoundInContainer
// when errdefs.IsNotFound reports true, and otherwise returns the original error.
func mapCopyErr(err error) error {
	if cerrdefs.IsNotFound(err) {
		return ErrFileNotFoundInContainer
	}
	return err
}

// firstFileFromTar reads the first regular file entry from a tar stream and
// returns its bytes (capped at 8 MB). Returns ErrFileNotFoundInContainer when
// no regular file entry is found.
func firstFileFromTar(r io.Reader) ([]byte, error) {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag == tar.TypeReg || hdr.Typeflag == 0 {
			const maxSize = 8 << 20 // 8 MB
			if hdr.Size > maxSize {
				return nil, ErrFileTooLargeInContainer // don't return a silently-truncated /etc/passwd
			}
			return io.ReadAll(io.LimitReader(tr, maxSize))
		}
	}
	return nil, ErrFileNotFoundInContainer
}

// CopyFromContainer reads a single file from the container filesystem and
// returns its raw bytes. It works on stopped and distroless containers (reads
// the image/RW layer). If the path does not exist,
// ErrFileNotFoundInContainer is returned.
func (c *Client) CopyFromContainer(ctx context.Context, id, srcPath string) ([]byte, error) {
	rc, _, err := c.cli.CopyFromContainer(ctx, id, srcPath)
	if err != nil {
		return nil, mapCopyErr(err)
	}
	defer rc.Close()
	return firstFileFromTar(rc)
}

// TopResult is the container's process list.
type TopResult struct {
	Titles    []string   `json:"titles"`
	Processes [][]string `json:"processes"`
}

// Top returns the running processes inside the container (equivalent to
// `docker top`). Returns an error for stopped containers — the daemon rejects
// the call; callers should guard on container state before calling.
func (c *Client) Top(ctx context.Context, id string) (TopResult, error) {
	resp, err := c.cli.ContainerTop(ctx, id, nil)
	if err != nil {
		return TopResult{}, err
	}
	processes := resp.Processes
	if processes == nil {
		processes = [][]string{}
	}
	return TopResult{
		Titles:    resp.Titles,
		Processes: processes,
	}, nil
}
