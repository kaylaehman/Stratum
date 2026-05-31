package stacks

import "testing"

func TestFirstConfigFile(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"single", "/data/compose/42/docker-compose.yml\n", "/data/compose/42/docker-compose.yml"},
		{"multi-line picks first non-empty", "\n\n/opt/app/compose.yml\n/opt/app/compose.yml\n", "/opt/app/compose.yml"},
		{"comma-joined takes first", "/srv/a.yml,/srv/b.yml\n", "/srv/a.yml"},
		{"no value sentinel skipped", "<no value>\n/x/docker-compose.yml\n", "/x/docker-compose.yml"},
		{"only no value", "<no value>\n<no value>\n", ""},
		{"whitespace trimmed", "   /q/compose.yaml  \n", "/q/compose.yaml"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := firstConfigFile(c.in); got != c.want {
				t.Fatalf("firstConfigFile(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestNonEmptyLines(t *testing.T) {
	got := nonEmptyLines("a\n\n  b \n\nc\n")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseMountLines(t *testing.T) {
	in := "/data\t/var/lib/docker/volumes/portainer_data/_data\n" +
		"/etc/localtime\t/etc/localtime\n" +
		"malformed-no-tab\n" +
		"\t/only-src\n"
	m := parseMountLines(in)
	if got := m["/data"]; got != "/var/lib/docker/volumes/portainer_data/_data" {
		t.Fatalf("/data -> %q", got)
	}
	if got := m["/etc/localtime"]; got != "/etc/localtime" {
		t.Fatalf("/etc/localtime -> %q", got)
	}
	if _, ok := m["malformed-no-tab"]; ok {
		t.Fatal("malformed line should be skipped")
	}
	if len(m) != 2 {
		t.Fatalf("map size = %d, want 2: %v", len(m), m)
	}
}

func TestRemapUnderMount(t *testing.T) {
	cases := []struct {
		name                      string
		containerPath, dest, src  string
		wantPath                  string
		wantOK                    bool
	}{
		{
			"portainer data dir", "/data/compose/42/docker-compose.yml",
			"/data", "/var/lib/docker/volumes/portainer_data/_data",
			"/var/lib/docker/volumes/portainer_data/_data/compose/42/docker-compose.yml", true,
		},
		{"exact mount", "/cfg", "/cfg", "/host/cfg", "/host/cfg", true},
		{"not under mount", "/data/x", "/other", "/host", "", false},
		{"root dest rejected", "/data/x", "/", "/host", "", false},
		{"empty src rejected", "/data/x", "/data", "", "", false},
		{"sibling prefix not matched", "/database/x", "/data", "/host", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := remapUnderMount(c.containerPath, c.dest, c.src)
			if ok != c.wantOK || got != c.wantPath {
				t.Fatalf("remapUnderMount(%q,%q,%q) = (%q,%v), want (%q,%v)",
					c.containerPath, c.dest, c.src, got, ok, c.wantPath, c.wantOK)
			}
		})
	}
}
