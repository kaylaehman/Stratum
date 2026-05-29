package api

import (
	"errors"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/kaylaehman/stratum/backend/docker"
	"github.com/kaylaehman/stratum/backend/fs"
)

// containerListCap bounds entries returned for one container directory.
const containerListCap = 10000

// cleanContainerPath normalizes a query path to an absolute, cleaned POSIX path,
// defaulting to "/".
func cleanContainerPath(p string) string {
	if strings.TrimSpace(p) == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return path.Clean(p)
}

// containerStatToEntry maps a raw container stat line to the shared fs.Entry
// shape so the frontend renders container files exactly like host files.
func containerStatToEntry(st docker.ContainerStat) fs.Entry {
	perm, _ := strconv.ParseUint(st.PermOctal, 8, 32)
	mode := os.FileMode(perm & 0o777)
	if perm&0o4000 != 0 {
		mode |= os.ModeSetuid
	}
	if perm&0o2000 != 0 {
		mode |= os.ModeSetgid
	}
	if perm&0o1000 != 0 {
		mode |= os.ModeSticky
	}
	switch st.Type {
	case "directory":
		mode |= os.ModeDir
	case "symbolic link":
		mode |= os.ModeSymlink
	}
	octal, rwx := fs.ModeStrings(mode)
	return fs.Entry{
		Name:      st.Name,
		IsDir:     st.Type == "directory",
		IsSymlink: st.Type == "symbolic link",
		Size:      st.Size,
		ModTime:   time.Unix(st.ModUnix, 0).UTC(),
		ModeOctal: octal,
		ModeRWX:   rwx,
		UID:       st.UID,
		GID:       st.GID,
		Owner:     st.Owner,
		Group:     st.Group,
		Classes:   fs.Classes(mode),
	}
}

// ContainerFSList lists a directory inside a container (operator+). Read-only:
// it runs `sh`/`stat` in the container via docker exec, so the container must be
// running and have a shell (distroless images return container_no_shell).
func (h *Handlers) ContainerFSList(w http.ResponseWriter, r *http.Request) {
	if !h.requireOperator(w, r) {
		return
	}
	ctr, clients, ok := h.resolveContainer(w, r)
	if !ok {
		return
	}
	p := cleanContainerPath(r.URL.Query().Get("path"))
	stats, err := clients.Docker.ListDirInContainer(r.Context(), ctr.DockerID, p)
	if errors.Is(err, docker.ErrNoShellInContainer) {
		writeError(w, http.StatusUnprocessableEntity, "container_no_shell")
		return
	} else if errors.Is(err, docker.ErrFileNotFoundInContainer) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusBadGateway, "container_unreachable")
		return
	}

	truncated := false
	if len(stats) > containerListCap {
		stats = stats[:containerListCap]
		truncated = true
	}
	entries := make([]fs.Entry, 0, len(stats))
	for _, st := range stats {
		entries = append(entries, containerStatToEntry(st))
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries, "truncated": truncated})
}

// ContainerFSFile reads a single file from a container (operator+). Works on
// stopped/distroless containers too (uses the archive API, not exec).
func (h *Handlers) ContainerFSFile(w http.ResponseWriter, r *http.Request) {
	if !h.requireOperator(w, r) {
		return
	}
	ctr, clients, ok := h.resolveContainer(w, r)
	if !ok {
		return
	}
	p := cleanContainerPath(r.URL.Query().Get("path"))
	data, err := clients.Docker.CopyFromContainer(r.Context(), ctr.DockerID, p)
	if errors.Is(err, docker.ErrFileTooLargeInContainer) {
		writeJSON(w, http.StatusOK, map[string]any{"too_large": true})
		return
	} else if errors.Is(err, docker.ErrFileNotFoundInContainer) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusBadGateway, "container_unreachable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"too_large": false, "content": string(data)})
}
