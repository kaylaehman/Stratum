package fs

import (
	"context"
	"io"
	"os"
	"path"

	"github.com/pkg/sftp"
	cssh "golang.org/x/crypto/ssh"

	appssh "github.com/kaylaehman/stratum/backend/ssh"
)

// sftpProvider implements FileProvider over a live SSH client using the
// per-operation SFTP ops in backend/ssh.
type sftpProvider struct {
	client *cssh.Client
}

// NewSFTPProvider wraps a live SSH client as a FileProvider.
func NewSFTPProvider(client *cssh.Client) FileProvider {
	return &sftpProvider{client: client}
}

func (p *sftpProvider) List(_ context.Context, dir string) ([]Entry, bool, error) {
	infos, err := appssh.SFTPReadDir(p.client, dir)
	if err != nil {
		return nil, false, err
	}
	truncated := false
	if len(infos) > listSoftCap {
		infos = infos[:listSoftCap]
		truncated = true
	}
	entries := make([]Entry, 0, len(infos))
	for _, fi := range infos {
		e := toEntry(fi)
		if e.IsSymlink {
			if target, err := appssh.SFTPReadLink(p.client, path.Join(dir, e.Name)); err == nil {
				e.LinkTarget = target
			}
		}
		entries = append(entries, e)
	}
	return entries, truncated, nil
}

func (p *sftpProvider) Stat(_ context.Context, pth string) (Entry, error) {
	fi, err := appssh.SFTPLstat(p.client, pth)
	if err != nil {
		return Entry{}, err
	}
	e := toEntry(fi)
	if e.IsSymlink {
		if target, err := appssh.SFTPReadLink(p.client, pth); err == nil {
			e.LinkTarget = target
		}
	}
	return e, nil
}

func (p *sftpProvider) RealPath(_ context.Context, pth string) (string, error) {
	return appssh.SFTPRealPath(p.client, pth)
}

func (p *sftpProvider) OpenRead(_ context.Context, pth string) (io.ReadCloser, error) {
	return appssh.SFTPOpenRead(p.client, pth)
}

func (p *sftpProvider) OpenWrite(_ context.Context, pth string) (io.WriteCloser, error) {
	return appssh.SFTPCreate(p.client, pth)
}

func (p *sftpProvider) Mkdir(_ context.Context, pth string) error {
	return appssh.SFTPMkdir(p.client, pth)
}

func (p *sftpProvider) Rename(_ context.Context, oldPath, newPath string) error {
	return appssh.SFTPRename(p.client, oldPath, newPath)
}

func (p *sftpProvider) Remove(_ context.Context, pth string, recursive bool) error {
	return appssh.SFTPRemove(p.client, pth, recursive)
}

// toEntry builds an Entry (sans resolved Owner/Group) from an SFTP FileInfo.
func toEntry(fi os.FileInfo) Entry {
	mode := fi.Mode()
	octal, rwx := ModeStrings(mode)
	e := Entry{
		Name:      fi.Name(),
		IsDir:     fi.IsDir(),
		IsSymlink: mode&os.ModeSymlink != 0,
		Size:      fi.Size(),
		ModTime:   fi.ModTime(),
		ModeOctal: octal,
		ModeRWX:   rwx,
		Classes:   Classes(mode),
	}
	if st, ok := fi.Sys().(*sftp.FileStat); ok {
		e.UID = int(st.UID)
		e.GID = int(st.GID)
	}
	return e
}
