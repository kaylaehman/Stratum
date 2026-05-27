package docker

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/errdefs"
)

// ---------------------------------------------------------------------------
// firstFileFromTar tests
// ---------------------------------------------------------------------------

// buildTar returns a tar archive containing one regular file with the given
// name and content.
func buildTar(t *testing.T, name string, content []byte) io.Reader {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	err := tw.WriteHeader(&tar.Header{
		Name:     name,
		Typeflag: tar.TypeReg,
		Size:     int64(len(content)),
		Mode:     0644,
	})
	if err != nil {
		t.Fatalf("tar write header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("tar write body: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	return &buf
}

// buildEmptyTar returns a tar archive with no entries.
func buildEmptyTar(t *testing.T) io.Reader {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	return &buf
}

func TestFirstFileFromTar_ReturnsBytes(t *testing.T) {
	want := []byte("root:x:0:0:root:/root:/bin/bash\n")
	r := buildTar(t, "etc/passwd", want)

	got, err := firstFileFromTar(r)
	if err != nil {
		t.Fatalf("firstFileFromTar unexpected error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("content mismatch: got %q, want %q", got, want)
	}
}

func TestFirstFileFromTar_EmptyTar_ReturnsNotFound(t *testing.T) {
	r := buildEmptyTar(t)
	_, err := firstFileFromTar(r)
	if !errors.Is(err, ErrFileNotFoundInContainer) {
		t.Errorf("expected ErrFileNotFoundInContainer, got %v", err)
	}
}

func TestFirstFileFromTar_DirectoryEntry_Skipped(t *testing.T) {
	// Build a tar with a directory entry followed by a regular file.
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// directory entry
	if err := tw.WriteHeader(&tar.Header{
		Name:     "etc/",
		Typeflag: tar.TypeDir,
		Mode:     0755,
	}); err != nil {
		t.Fatalf("write dir header: %v", err)
	}

	// regular file entry
	content := []byte("file content")
	if err := tw.WriteHeader(&tar.Header{
		Name:     "etc/file.txt",
		Typeflag: tar.TypeReg,
		Size:     int64(len(content)),
		Mode:     0644,
	}); err != nil {
		t.Fatalf("write file header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("write file body: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}

	got, err := firstFileFromTar(&buf)
	if err != nil {
		t.Fatalf("firstFileFromTar unexpected error: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
}

// ---------------------------------------------------------------------------
// mapCopyErr tests
// ---------------------------------------------------------------------------

func TestMapCopyErr_NotFound_ReturnsErrFileNotFoundInContainer(t *testing.T) {
	notFound := errdefs.NotFound(errors.New("no such file"))
	got := mapCopyErr(notFound)
	if !errors.Is(got, ErrFileNotFoundInContainer) {
		t.Errorf("expected ErrFileNotFoundInContainer, got %v", got)
	}
}

func TestMapCopyErr_OtherError_Passthrough(t *testing.T) {
	other := errors.New("some other error")
	got := mapCopyErr(other)
	if !errors.Is(got, other) {
		t.Errorf("expected original error %v, got %v", other, got)
	}
}

func TestMapCopyErr_Nil_ReturnsNil(t *testing.T) {
	if got := mapCopyErr(nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// mapMounts tests
// ---------------------------------------------------------------------------

func TestMapMounts_EmptySlice(t *testing.T) {
	mounts := mapMounts(nil)
	if len(mounts) != 0 {
		t.Errorf("expected empty slice, got %d elements", len(mounts))
	}
}

func TestMapMounts_BindMount(t *testing.T) {
	input := []container.MountPoint{
		{
			Type:        mount.TypeBind,
			Source:      "/host/data",
			Destination: "/data",
			Mode:        "rw",
			RW:          true,
		},
	}
	got := mapMounts(input)
	if len(got) != 1 {
		t.Fatalf("expected 1 mount, got %d", len(got))
	}
	m := got[0]
	if m.Type != "bind" {
		t.Errorf("Type: got %q, want %q", m.Type, "bind")
	}
	if m.Source != "/host/data" {
		t.Errorf("Source: got %q, want %q", m.Source, "/host/data")
	}
	if m.Destination != "/data" {
		t.Errorf("Destination: got %q, want %q", m.Destination, "/data")
	}
	if !m.RW {
		t.Error("RW: expected true, got false")
	}
}

func TestMapMounts_VolumeMount(t *testing.T) {
	input := []container.MountPoint{
		{
			Type:        mount.TypeVolume,
			Name:        "myvolume",
			Source:      "/var/lib/docker/volumes/myvolume/_data",
			Destination: "/var/lib/data",
			Mode:        "",
			RW:          true,
		},
	}
	got := mapMounts(input)
	if got[0].Type != "volume" {
		t.Errorf("Type: got %q, want %q", got[0].Type, "volume")
	}
}

// ---------------------------------------------------------------------------
// mapInspect tests
// ---------------------------------------------------------------------------

func TestMapInspect_StripLeadingSlash(t *testing.T) {
	r := container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			ID:   "abc123",
			Name: "/mycontainer",
			State: &container.State{
				Status: "running",
			},
			HostConfig: &container.HostConfig{},
		},
		Config: &container.Config{},
	}
	info := mapInspect(r)
	if info.Name != "mycontainer" {
		t.Errorf("Name: got %q, want %q", info.Name, "mycontainer")
	}
}

func TestMapInspect_StateAndImage(t *testing.T) {
	r := container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			ID:    "def456",
			Name:  "/webserver",
			Image: "sha256:deadbeef",
			State: &container.State{
				Status: "exited",
			},
			HostConfig: &container.HostConfig{},
		},
		Config: &container.Config{
			Image: "nginx:latest",
			User:  "1000:1000",
		},
	}
	info := mapInspect(r)
	if info.State != "exited" {
		t.Errorf("State: got %q, want %q", info.State, "exited")
	}
	if info.Image != "nginx:latest" {
		t.Errorf("Image: got %q, want %q", info.Image, "nginx:latest")
	}
	if info.ImageID != "sha256:deadbeef" {
		t.Errorf("ImageID: got %q, want %q", info.ImageID, "sha256:deadbeef")
	}
	if info.ConfigUser != "1000:1000" {
		t.Errorf("ConfigUser: got %q, want %q", info.ConfigUser, "1000:1000")
	}
}

func TestMapInspect_PrivilegedAndCaps(t *testing.T) {
	r := container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			ID:   "priv1",
			Name: "/privileged-ctr",
			State: &container.State{
				Status: "running",
			},
			HostConfig: &container.HostConfig{
				Privileged: true,
				CapAdd:     []string{"CAP_SYS_ADMIN", "CAP_NET_ADMIN"},
				CapDrop:    []string{"CAP_MKNOD"},
			},
		},
		Config: &container.Config{},
	}
	info := mapInspect(r)
	if !info.Privileged {
		t.Error("Privileged: expected true")
	}
	if len(info.CapAdd) != 2 || info.CapAdd[0] != "CAP_SYS_ADMIN" {
		t.Errorf("CapAdd: got %v", info.CapAdd)
	}
	if len(info.CapDrop) != 1 || info.CapDrop[0] != "CAP_MKNOD" {
		t.Errorf("CapDrop: got %v", info.CapDrop)
	}
}

func TestMapInspect_PidAndNetworkMode(t *testing.T) {
	r := container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			ID:   "net1",
			Name: "/net-ctr",
			State: &container.State{
				Status: "running",
			},
			HostConfig: &container.HostConfig{
				PidMode:     container.PidMode("host"),
				NetworkMode: container.NetworkMode("host"),
			},
		},
		Config: &container.Config{},
	}
	info := mapInspect(r)
	if info.PidMode != "host" {
		t.Errorf("PidMode: got %q, want %q", info.PidMode, "host")
	}
	if info.NetworkMode != "host" {
		t.Errorf("NetworkMode: got %q, want %q", info.NetworkMode, "host")
	}
}

func TestMapInspect_RepoDigestsAlwaysEmpty(t *testing.T) {
	r := container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			ID:         "rd1",
			Name:       "/c",
			State:      &container.State{Status: "running"},
			HostConfig: &container.HostConfig{},
		},
		Config: &container.Config{},
	}
	info := mapInspect(r)
	if info.RepoDigests == nil {
		t.Error("RepoDigests: expected non-nil empty slice, got nil")
	}
	if len(info.RepoDigests) != 0 {
		t.Errorf("RepoDigests: expected empty, got %v", info.RepoDigests)
	}
}

func TestMapInspect_NilHostConfig_DoesNotPanic(t *testing.T) {
	r := container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			ID:         "nilhc",
			Name:       "/c",
			State:      &container.State{Status: "running"},
			HostConfig: nil,
		},
		Config: &container.Config{},
	}
	// Should not panic; CapAdd/CapDrop should be empty slices.
	info := mapInspect(r)
	if info.CapAdd == nil {
		t.Error("CapAdd: expected non-nil empty slice for nil HostConfig")
	}
	if info.CapDrop == nil {
		t.Error("CapDrop: expected non-nil empty slice for nil HostConfig")
	}
}

func TestMapInspect_NilState_DoesNotPanic(t *testing.T) {
	r := container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			ID:         "nilstate",
			Name:       "/c",
			State:      nil,
			HostConfig: &container.HostConfig{},
		},
		Config: &container.Config{},
	}
	info := mapInspect(r)
	if info.State != "" {
		t.Errorf("State: expected empty string for nil State, got %q", info.State)
	}
}

// ---------------------------------------------------------------------------
// Compile-only verification: Inspect, CopyFromContainer, Top on *Client
// ---------------------------------------------------------------------------

// The functions below assert that the public API surface compiles exactly as
// specified. They reference unexported helpers used by those methods.

var _ = func() {
	var c *Client
	var _ = c.Inspect
	var _ = c.CopyFromContainer
	var _ = c.Top
	_ = strings.TrimPrefix // suppress unused import
}
