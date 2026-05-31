// Package orchestration builds dependency-ordered execution plans for stacks
// and nodes, and executes them against live Docker/Proxmox lifecycle APIs.
package orchestration

import (
	"sort"
)

// topoSort returns nodes in topological order (dependencies first).
// edges maps node → set of nodes it depends on (its dependencies).
// Returns (sorted, cycles) where cycles is a list of SCCs with size > 1.
func topoSort(nodes []string, deps map[string][]string) (sorted []string, cycles [][]string) {
	index := map[string]int{}
	lowlink := map[string]int{}
	onStack := map[string]bool{}
	var stack []string
	var sccs [][]string
	counter := 0

	var strongConnect func(v string)
	strongConnect = func(v string) {
		index[v] = counter
		lowlink[v] = counter
		counter++
		stack = append(stack, v)
		onStack[v] = true

		for _, w := range deps[v] {
			if _, seen := index[w]; !seen {
				strongConnect(w)
				if lowlink[w] < lowlink[v] {
					lowlink[v] = lowlink[w]
				}
			} else if onStack[w] {
				if index[w] < lowlink[v] {
					lowlink[v] = index[w]
				}
			}
		}

		if lowlink[v] == index[v] {
			var scc []string
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				scc = append(scc, w)
				if w == v {
					break
				}
			}
			sccs = append(sccs, scc)
		}
	}

	for _, n := range nodes {
		if _, seen := index[n]; !seen {
			strongConnect(n)
		}
	}

	// Extract cycles (SCCs with >1 member).
	for _, scc := range sccs {
		if len(scc) > 1 {
			cycles = append(cycles, scc)
		}
	}

	// Tarjan's algorithm produces SCCs in reverse topological order of the
	// condensation. For edges meaning "A depends on B" (A→B), the SCC
	// containing only leaf nodes (no outgoing deps) is produced first.
	// This is the dependency-first order we want for "start" plans.
	// We do NOT reverse; the raw SCC list is already correct.
	seen := map[string]bool{}
	for _, scc := range sccs {
		members := make([]string, len(scc))
		copy(members, scc)
		sort.Strings(members) // stable order within SCC
		for _, m := range members {
			if !seen[m] {
				seen[m] = true
				sorted = append(sorted, m)
			}
		}
	}
	return sorted, cycles
}
