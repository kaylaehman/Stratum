package bulk

import (
	"testing"

	"github.com/kaylaehman/stratum/backend/db"
)

func TestValid(t *testing.T) {
	for _, a := range []string{ActionStart, ActionStop, ActionRestart, ActionRemove} {
		if !Valid(a) {
			t.Errorf("Valid(%q) = false, want true", a)
		}
	}
	for _, a := range []string{"", "pull", "delete", "START"} {
		if Valid(a) {
			t.Errorf("Valid(%q) = true, want false", a)
		}
	}
}

func TestPlanRemoveSkipsRunning(t *testing.T) {
	containers := []db.Container{
		{ID: "c1", Name: "a", NodeID: "n1", DockerID: "d1", Status: "running"},
		{ID: "c2", Name: "b", NodeID: "n1", DockerID: "d2", Status: "exited"},
	}
	items := Plan(ActionRemove, containers)
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if !items[0].Skip || items[0].SkipReason == "" {
		t.Errorf("running container should be skipped for remove: %+v", items[0])
	}
	if items[1].Skip {
		t.Errorf("stopped container should NOT be skipped for remove: %+v", items[1])
	}
}

func TestPlanLifecycleNeverSkips(t *testing.T) {
	containers := []db.Container{
		{ID: "c1", Status: "running"},
		{ID: "c2", Status: "exited"},
	}
	for _, action := range []string{ActionStart, ActionStop, ActionRestart} {
		for _, it := range Plan(action, containers) {
			if it.Skip {
				t.Errorf("%s should not skip any container, but skipped %s", action, it.ContainerID)
			}
		}
	}
}
