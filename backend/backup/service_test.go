package backup

import "testing"

func TestValidVolume(t *testing.T) {
	for _, v := range []string{"plex_config", "my-vol", "data.1", "abc123"} {
		if !ValidVolume(v) {
			t.Errorf("%q should be valid", v)
		}
	}
	for _, v := range []string{"", "a b", "vol;rm", "$(x)", "../etc", "a/b"} {
		if ValidVolume(v) {
			t.Errorf("%q should be rejected", v)
		}
	}
}

func TestValidDestDir(t *testing.T) {
	if !ValidDestDir("/mnt/backups") || !ValidDestDir("/opt/x") {
		t.Error("absolute dirs should be valid")
	}
	for _, d := range []string{"relative", "/mnt/../etc", ""} {
		if ValidDestDir(d) {
			t.Errorf("%q should be rejected", d)
		}
	}
}

func TestArchiveName(t *testing.T) {
	if got := archiveName("/mnt/backups/plex-123.tar.gz"); got != "plex-123.tar.gz" {
		t.Errorf("archiveName = %q", got)
	}
}
