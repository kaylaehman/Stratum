package docker

import "testing"

func TestForwardExposure(t *testing.T) {
	mounts := []Mount{
		{Type: "bind", Source: "/srv/media", Destination: "/media", RW: true},
		{Type: "bind", Source: "/srv/config", Destination: "/config", RW: false},
		{Type: "volume", Source: "/var/lib/docker/volumes/appdata/_data", Destination: "/data", RW: true},
	}

	t.Run("file under bind source", func(t *testing.T) {
		e := Forward("/srv/media/movies/x.mkv", mounts)
		if !e.Exposed || e.ContainerPath != "/media/movies/x.mkv" || !e.RW {
			t.Fatalf("got %+v", e)
		}
	})
	t.Run("exact source", func(t *testing.T) {
		e := Forward("/srv/config", mounts)
		if !e.Exposed || e.ContainerPath != "/config" || e.RW {
			t.Fatalf("got %+v", e)
		}
	})
	t.Run("segment-aware: /srv/media-archive does NOT match /srv/media", func(t *testing.T) {
		e := Forward("/srv/media-archive/x", mounts)
		if e.Exposed {
			t.Fatalf("/srv/media-archive must not match /srv/media: %+v", e)
		}
	})
	t.Run("not exposed", func(t *testing.T) {
		if Forward("/home/kayla/secret", mounts).Exposed {
			t.Fatal("unmounted path should not be exposed")
		}
	})
	t.Run("named volume", func(t *testing.T) {
		e := Forward("/var/lib/docker/volumes/appdata/_data/db.sqlite", mounts)
		if !e.Exposed || !e.IsNamedVolume || e.VolumeName != "appdata" || e.ContainerPath != "/data/db.sqlite" {
			t.Fatalf("got %+v", e)
		}
	})
	t.Run("most specific wins", func(t *testing.T) {
		nested := []Mount{
			{Type: "bind", Source: "/srv", Destination: "/srv-mount", RW: true},
			{Type: "bind", Source: "/srv/media", Destination: "/media", RW: false},
		}
		e := Forward("/srv/media/x", nested)
		if e.ViaSource != "/srv/media" || e.ContainerPath != "/media/x" {
			t.Fatalf("most specific mount should win, got %+v", e)
		}
	})
}
