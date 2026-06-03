package updates

import "testing"

func TestDetectManagers(t *testing.T) {
	cases := []struct {
		name        string
		images      []string
		wantNames   []string
		autoUpdater bool
	}{
		{"none", []string{"nginx:latest", "jellyfin/jellyfin"}, nil, false},
		{"watchtower", []string{"containrrr/watchtower:latest"}, []string{"watchtower"}, true},
		{"watchtower bare", []string{"watchtower"}, []string{"watchtower"}, true},
		{"portainer ce", []string{"portainer/portainer-ce:2.19"}, []string{"portainer"}, false},
		{"portainer agent", []string{"portainer/agent"}, []string{"portainer"}, false},
		{"both sorted+deduped", []string{"portainer/portainer-ce", "containrrr/watchtower", "portainer/agent"}, []string{"portainer", "watchtower"}, true},
		{"case-insensitive", []string{"Containrrr/WatchTower:latest"}, []string{"watchtower"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DetectManagers(tc.images)
			if len(got) != len(tc.wantNames) {
				t.Fatalf("DetectManagers(%v) = %+v, want names %v", tc.images, got, tc.wantNames)
			}
			for i, n := range tc.wantNames {
				if got[i].Name != n {
					t.Errorf("manager[%d] = %q, want %q", i, got[i].Name, n)
				}
			}
			if HasAutoUpdater(got) != tc.autoUpdater {
				t.Errorf("HasAutoUpdater = %v, want %v", HasAutoUpdater(got), tc.autoUpdater)
			}
		})
	}
}
