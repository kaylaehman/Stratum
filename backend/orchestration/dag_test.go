package orchestration

import (
	"reflect"
	"sort"
	"testing"
)

func TestTopoSort_Linear(t *testing.T) {
	// A depends on B, B depends on C: expected order C, B, A
	nodes := []string{"A", "B", "C"}
	deps := map[string][]string{
		"A": {"B"},
		"B": {"C"},
		"C": nil,
	}
	sorted, cycles := topoSort(nodes, deps)
	if len(cycles) != 0 {
		t.Fatalf("expected no cycles, got %v", cycles)
	}
	if len(sorted) != 3 {
		t.Fatalf("expected 3 items, got %d: %v", len(sorted), sorted)
	}
	// C before B, B before A
	pos := map[string]int{}
	for i, n := range sorted {
		pos[n] = i
	}
	if pos["C"] > pos["B"] {
		t.Errorf("C should come before B, got order %v", sorted)
	}
	if pos["B"] > pos["A"] {
		t.Errorf("B should come before A, got order %v", sorted)
	}
}

func TestTopoSort_NoDeps(t *testing.T) {
	nodes := []string{"X", "Y", "Z"}
	deps := map[string][]string{"X": nil, "Y": nil, "Z": nil}
	sorted, cycles := topoSort(nodes, deps)
	if len(cycles) != 0 {
		t.Fatalf("expected no cycles, got %v", cycles)
	}
	// All three must appear, any order is valid.
	have := make([]string, len(sorted))
	copy(have, sorted)
	sort.Strings(have)
	want := []string{"X", "Y", "Z"}
	sort.Strings(want)
	if !reflect.DeepEqual(have, want) {
		t.Errorf("expected all nodes, got %v", sorted)
	}
}

func TestTopoSort_Cycle(t *testing.T) {
	// A → B → C → A (cycle)
	nodes := []string{"A", "B", "C"}
	deps := map[string][]string{
		"A": {"B"},
		"B": {"C"},
		"C": {"A"},
	}
	sorted, cycles := topoSort(nodes, deps)
	if len(cycles) == 0 {
		t.Fatal("expected a cycle to be detected")
	}
	// All nodes still returned (sorted within cycle by stable order).
	if len(sorted) != 3 {
		t.Errorf("expected 3 items even with cycle, got %d: %v", len(sorted), sorted)
	}
}

func TestTopoSort_PartialCycle(t *testing.T) {
	// D → A → B → A (A↔B cycle, D is outside)
	nodes := []string{"A", "B", "C", "D"}
	deps := map[string][]string{
		"A": {"B"},
		"B": {"A"},
		"C": nil,
		"D": {"A"},
	}
	sorted, cycles := topoSort(nodes, deps)
	if len(cycles) == 0 {
		t.Fatal("expected a cycle")
	}
	// D and C must appear; A and B in cycle.
	have := make([]string, len(sorted))
	copy(have, sorted)
	sort.Strings(have)
	want := []string{"A", "B", "C", "D"}
	sort.Strings(want)
	if !reflect.DeepEqual(have, want) {
		t.Errorf("expected all nodes, got %v", sorted)
	}
}

func TestTopoSort_Diamond(t *testing.T) {
	// D depends on B and C; B and C both depend on A
	nodes := []string{"A", "B", "C", "D"}
	deps := map[string][]string{
		"D": {"B", "C"},
		"B": {"A"},
		"C": {"A"},
		"A": nil,
	}
	sorted, cycles := topoSort(nodes, deps)
	if len(cycles) != 0 {
		t.Fatalf("expected no cycles, got %v", cycles)
	}
	pos := map[string]int{}
	for i, n := range sorted {
		pos[n] = i
	}
	if pos["A"] > pos["B"] {
		t.Errorf("A should come before B")
	}
	if pos["A"] > pos["C"] {
		t.Errorf("A should come before C")
	}
	if pos["B"] > pos["D"] {
		t.Errorf("B should come before D")
	}
	if pos["C"] > pos["D"] {
		t.Errorf("C should come before D")
	}
}

func TestReverse(t *testing.T) {
	s := []string{"A", "B", "C"}
	reverse(s)
	want := []string{"C", "B", "A"}
	if !reflect.DeepEqual(s, want) {
		t.Errorf("expected %v, got %v", want, s)
	}
}

func TestReverse_Empty(t *testing.T) {
	s := []string{}
	reverse(s) // must not panic
}
