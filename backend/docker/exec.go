package docker

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

// ErrNoShellInContainer is returned when the container can't run the listing
// command (no /bin/sh or no stat — e.g. a distroless image). Callers should
// suggest browsing the host path via the bind-mount tracer instead.
var ErrNoShellInContainer = errors.New("docker: container cannot list files (no shell)")

const maxExecOutput = 8 << 20

// ContainerStat is one raw directory entry emitted by `stat -c`. The API layer
// maps it to the shared fs.Entry shape (mode strings, classes).
type ContainerStat struct {
	Name      string
	Type      string // %F: "directory" | "symbolic link" | "regular file" | ...
	PermOctal string // %a, e.g. "644" or "4755"
	UID       int
	GID       int
	Owner     string
	Group     string
	Size      int64
	ModUnix   int64 // %Y epoch seconds
}

// containerLsScript lists the immediate children of $1 with stat metadata, one
// "|"-delimited line per entry. Portable across busybox and coreutils. The path
// is passed as a positional parameter ($1) — never interpolated into the script
// text — so it can't break out of the command. Exit 2 means the dir is missing.
const containerLsScript = `cd "$1" 2>/dev/null || exit 2
for e in * .* ; do
  [ "$e" = "." ] && continue
  [ "$e" = ".." ] && continue
  [ -e "$e" ] || [ -L "$e" ] || continue
  stat -c '%F|%s|%a|%u|%g|%U|%G|%Y|%n' -- "$e" 2>/dev/null
done`

// ExecCapture runs argv inside a (running) container and returns stdout and the
// process exit code. stderr is demultiplexed and discarded.
func (c *Client) ExecCapture(ctx context.Context, id string, argv []string) (stdout string, exitCode int, err error) {
	created, err := c.cli.ContainerExecCreate(ctx, id, container.ExecOptions{
		Cmd:          argv,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return "", 0, err
	}
	att, err := c.cli.ContainerExecAttach(ctx, created.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", 0, err
	}
	defer att.Close()

	var out, errb bytes.Buffer
	if _, err := stdcopy.StdCopy(&out, &errb, io.LimitReader(att.Reader, maxExecOutput)); err != nil && !errors.Is(err, io.EOF) {
		return "", 0, err
	}
	insp, err := c.cli.ContainerExecInspect(ctx, created.ID)
	if err != nil {
		return out.String(), 0, err
	}
	return out.String(), insp.ExitCode, nil
}

// ListDirInContainer lists the immediate children of path inside the container.
// Requires a running container with a shell + stat.
func (c *Client) ListDirInContainer(ctx context.Context, id, path string) ([]ContainerStat, error) {
	out, code, err := c.ExecCapture(ctx, id, []string{"sh", "-c", containerLsScript, "stratum", path})
	if err != nil {
		if isMissingBinaryErr(err) {
			return nil, ErrNoShellInContainer
		}
		return nil, err
	}
	switch code {
	case 2:
		return nil, ErrFileNotFoundInContainer
	case 126, 127:
		return nil, ErrNoShellInContainer
	}
	return parseContainerStat(out), nil
}

func isMissingBinaryErr(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "executable file not found") ||
		strings.Contains(s, "no such file or directory") && strings.Contains(s, "exec")
}

// parseContainerStat parses the "|"-delimited stat lines. Malformed lines are
// skipped. The name (last field) may itself contain "|", so it is kept whole.
func parseContainerStat(raw string) []ContainerStat {
	var out []ContainerStat
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		f := strings.SplitN(line, "|", 9)
		if len(f) != 9 {
			continue
		}
		size, _ := strconv.ParseInt(f[1], 10, 64)
		uid, _ := strconv.Atoi(f[3])
		gid, _ := strconv.Atoi(f[4])
		mod, _ := strconv.ParseInt(f[7], 10, 64)
		out = append(out, ContainerStat{
			Type:      f[0],
			Size:      size,
			PermOctal: f[2],
			UID:       uid,
			GID:       gid,
			Owner:     f[5],
			Group:     f[6],
			ModUnix:   mod,
			Name:      f[8],
		})
	}
	return out
}
