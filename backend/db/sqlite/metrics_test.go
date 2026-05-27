package sqlite_test

import (
	"context"
	"testing"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

func TestResourceSamples(t *testing.T) {
	ctx := context.Background()
	st, _ := newStoreWithDB(t)

	if err := st.CreateNode(ctx, appdb.Node{ID: "n1", Name: "h", Type: "standalone", Host: "10.0.0.1", Port: 22, CapabilitiesJSON: "{}", Status: "unknown", CredentialsEncrypted: []byte("x")}); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertContainer(ctx, appdb.Container{ID: "c1", NodeID: "n1", DockerID: "deadbeef", Name: "app", Image: "nginx", Status: "running"}); err != nil {
		t.Fatalf("UpsertContainer: %v", err)
	}

	base := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		s := appdb.ResourceSample{
			ID: "s" + string(rune('1'+i)), ContainerID: "c1", NodeID: "n1",
			CPUPct: float64(i * 10), MemBytes: int64(i * 1000), MemLimitBytes: 100000,
			DiskReadBytes: int64(i), DiskWriteBytes: int64(i * 2),
			SampledAt: base.Add(time.Duration(i) * time.Minute),
		}
		if err := st.InsertResourceSample(ctx, s); err != nil {
			t.Fatalf("InsertResourceSample %d: %v", i, err)
		}
	}

	// Range covering minutes 1..3 (inclusive) => 3 samples, ascending.
	from := base.Add(1 * time.Minute)
	to := base.Add(3 * time.Minute)
	got, err := st.ListResourceSamples(ctx, "c1", from, to)
	if err != nil {
		t.Fatalf("ListResourceSamples: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("range len = %d, want 3", len(got))
	}
	if got[0].CPUPct != 10 || got[2].CPUPct != 30 {
		t.Errorf("range order wrong: %v .. %v", got[0].CPUPct, got[2].CPUPct)
	}

	// Prune everything before minute 4 => only the last 2 remain.
	if _, err := st.PruneResourceSamplesBefore(ctx, base.Add(3*time.Minute+30*time.Second)); err != nil {
		t.Fatalf("prune: %v", err)
	}
	all, _ := st.ListResourceSamples(ctx, "c1", base, base.Add(time.Hour))
	if len(all) != 1 {
		t.Errorf("after prune got %d, want 1 (only minute-4 sample)", len(all))
	}
}
