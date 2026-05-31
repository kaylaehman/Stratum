package volumes

import (
	"errors"
	"testing"
)

func TestStatusClassification(t *testing.T) {
	cases := []struct {
		refCount int64
		attached int
		want     string
	}{
		{2, 2, StatusAttached},  // daemon + mount index agree
		{1, 0, StatusAttached},  // daemon says attached even if mount index lagged
		{0, 1, StatusAttached},  // mount index sees an attachment df missed
		{0, 0, StatusUnused},    // genuinely unused
		{-1, 0, StatusUnknown},  // daemon couldn't report a refcount
		{-1, 2, StatusAttached}, // unknown refcount but mounts present => attached
	}
	for _, c := range cases {
		if got := status(c.refCount, c.attached); got != c.want {
			t.Errorf("status(ref=%d, attached=%d) = %q, want %q", c.refCount, c.attached, got, c.want)
		}
	}
}

func TestPruneVolumes(t *testing.T) {
	mk := func(name, status string) VolumeView {
		return VolumeView{NodeID: "n1", Name: name, Status: status}
	}

	t.Run("only unused are removed", func(t *testing.T) {
		vols := []VolumeView{
			mk("keep1", StatusAttached),
			mk("drop1", StatusUnused),
			mk("keep2", StatusUnknown),
			mk("drop2", StatusUnused),
		}
		var removed []string
		res := pruneVolumes("n1", vols, func(name string) error {
			removed = append(removed, name)
			return nil
		})
		if len(res) != 2 {
			t.Fatalf("expected 2 results (unused only), got %d: %+v", len(res), res)
		}
		// Attached/unknown volumes must never be passed to remove.
		if len(removed) != 2 || removed[0] != "drop1" || removed[1] != "drop2" {
			t.Fatalf("remove called with %v, want [drop1 drop2]", removed)
		}
		for _, r := range res {
			if !r.OK || r.Error != "" {
				t.Errorf("result %+v should be OK with no error", r)
			}
			if r.NodeID != "n1" {
				t.Errorf("node_id propagation: got %q", r.NodeID)
			}
		}
	})

	t.Run("error mid-batch does not abort", func(t *testing.T) {
		vols := []VolumeView{
			mk("a", StatusUnused),
			mk("b", StatusUnused), // this one flips to in-use mid-batch
			mk("c", StatusUnused),
		}
		boom := errors.New("volume is in use")
		var attempted []string
		res := pruneVolumes("n1", vols, func(name string) error {
			attempted = append(attempted, name)
			if name == "b" {
				return boom
			}
			return nil
		})
		// All three must be attempted (batch never aborts on a single failure).
		if len(attempted) != 3 {
			t.Fatalf("expected all 3 attempted, got %v", attempted)
		}
		if len(res) != 3 {
			t.Fatalf("expected 3 results, got %d", len(res))
		}
		byName := map[string]PruneResult{}
		for _, r := range res {
			byName[r.Name] = r
		}
		if !byName["a"].OK || !byName["c"].OK {
			t.Errorf("a and c should be OK: %+v", res)
		}
		if byName["b"].OK || byName["b"].Error != boom.Error() {
			t.Errorf("b should be OK:false with the error, got %+v", byName["b"])
		}
	})

	t.Run("empty set yields empty non-nil slice", func(t *testing.T) {
		res := pruneVolumes("n1", nil, func(string) error {
			t.Fatal("remove must not be called for an empty set")
			return nil
		})
		if res == nil {
			t.Fatal("result should be a non-nil empty slice (marshals to [] not null)")
		}
		if len(res) != 0 {
			t.Fatalf("expected 0 results, got %d", len(res))
		}
	})

	t.Run("all attached yields no removals", func(t *testing.T) {
		vols := []VolumeView{mk("x", StatusAttached), mk("y", StatusAttached)}
		res := pruneVolumes("n1", vols, func(string) error {
			t.Fatal("remove must not be called when nothing is unused")
			return nil
		})
		if len(res) != 0 {
			t.Fatalf("expected 0 results, got %d", len(res))
		}
	})
}

func TestAppendUnique(t *testing.T) {
	s := appendUnique(nil, "a")
	s = appendUnique(s, "b")
	s = appendUnique(s, "a") // dup ignored
	if len(s) != 2 || s[0] != "a" || s[1] != "b" {
		t.Errorf("appendUnique = %v, want [a b]", s)
	}
}
