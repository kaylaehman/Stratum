package sqlite_test

import (
	"context"
	"testing"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

func TestVolumeSamples(t *testing.T) {
	ctx := context.Background()
	st, _ := newStoreWithDB(t)

	// A node FK is required (CASCADE on delete); create one via the nodes path is
	// heavy, so insert directly is not possible — use the node store.
	node := appdb.Node{ID: "n1", Name: "h", Type: "standalone", Host: "10.0.0.1", Port: 22, CapabilitiesJSON: "{}", Status: "unknown", CredentialsEncrypted: []byte("x")}
	if err := st.CreateNode(ctx, node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	base := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	samples := []appdb.VolumeSample{
		{ID: "s1", NodeID: "n1", VolumeName: "data", SizeBytes: 100, RefCount: 1, SampledAt: base},
		{ID: "s2", NodeID: "n1", VolumeName: "data", SizeBytes: 150, RefCount: 1, SampledAt: base.AddDate(0, 0, 1)},
		{ID: "s3", NodeID: "n1", VolumeName: "cache", SizeBytes: 5, RefCount: 0, SampledAt: base},
	}
	for _, s := range samples {
		if err := st.InsertVolumeSample(ctx, s); err != nil {
			t.Fatalf("InsertVolumeSample %s: %v", s.ID, err)
		}
	}

	got, err := st.ListVolumeSamplesByNode(ctx, "n1")
	if err != nil {
		t.Fatalf("ListVolumeSamplesByNode: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d samples, want 3", len(got))
	}
	// Collect the "data" volume's trend in returned (sampled_at ASC) order.
	var dataTrend []int64
	var sawCache bool
	for _, s := range got {
		switch s.VolumeName {
		case "data":
			dataTrend = append(dataTrend, s.SizeBytes)
		case "cache":
			sawCache = true
		}
	}
	if len(dataTrend) != 2 || dataTrend[0] != 100 || dataTrend[1] != 150 {
		t.Errorf("data trend = %v, want [100 150] (ascending by time)", dataTrend)
	}
	if !sawCache {
		t.Error("cache volume sample missing")
	}
}
