package topology

import (
	"testing"

	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/docker"
)

func TestBuildContainerNodes(t *testing.T) {
	networks := []docker.NetworkInfo{
		{Name: "bridge", Endpoints: []docker.NetworkEndpoint{{ContainerID: "c1"}}},
		{Name: "backend", Endpoints: []docker.NetworkEndpoint{{ContainerID: "c1"}, {ContainerID: "c2"}}},
		{Name: "host", Endpoints: []docker.NetworkEndpoint{{ContainerID: "c3"}}},
	}
	containers := []db.Container{
		{DockerID: "c1", Name: "web", Status: "running"},
		{DockerID: "c2", Name: "db", Status: "running"},
		{DockerID: "c3", Name: "monitor", Status: "running"},
		{DockerID: "c4", Name: "lonely", Status: "exited"},
	}

	got := buildContainerNodes(networks, containers)
	byName := map[string]ContainerNode{}
	for _, c := range got {
		byName[c.Name] = c
	}

	// c1 is on two networks (sorted), not isolated, not host.
	web := byName["web"]
	if len(web.Networks) != 2 || web.Networks[0] != "backend" || web.Networks[1] != "bridge" {
		t.Errorf("web networks = %v, want [backend bridge]", web.Networks)
	}
	if web.Isolated || web.HostNetwork {
		t.Errorf("web should be neither isolated nor host: %+v", web)
	}
	// c3 is on the host network.
	if !byName["monitor"].HostNetwork {
		t.Error("monitor should be flagged host_network")
	}
	// c4 is in no network => isolated.
	if !byName["lonely"].Isolated || len(byName["lonely"].Networks) != 0 {
		t.Errorf("lonely should be isolated with no networks: %+v", byName["lonely"])
	}
	// Output sorted by name.
	if got[0].Name != "db" {
		t.Errorf("first (sorted) = %s, want db", got[0].Name)
	}
}
