package api

import (
	"net/http"
	"strings"

	"github.com/KAE-Labs/stratum/backend/middleware"
)

// searchGroupLimit caps how many hits per category are returned.
const searchGroupLimit = 10

// matchesQuery reports whether the lower-cased query is a substring of any field
// (case-insensitive). An empty query never matches.
func matchesQuery(q string, fields ...string) bool {
	if q == "" {
		return false
	}
	lq := strings.ToLower(q)
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), lq) {
			return true
		}
	}
	return false
}

type searchNodeHit struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}
type searchContainerHit struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	NodeID string `json:"node_id"`
	Image  string `json:"image"`
	Status string `json:"status"`
}
type searchVMHit struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	NodeID string `json:"node_id"`
}
type searchBookmarkHit struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	ResourceType string `json:"resource_type"`
	ResourceRef  string `json:"resource_ref"`
}

// Search is a global, case-insensitive substring search across the persisted
// inventory (nodes, containers, VMs) and the caller's bookmarks. Results are
// grouped by type and capped per group. Read-only.
func (h *Handlers) Search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	nodes := []searchNodeHit{}
	containers := []searchContainerHit{}
	vms := []searchVMHit{}
	bookmarks := []searchBookmarkHit{}

	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"nodes": nodes, "containers": containers, "vms": vms, "bookmarks": bookmarks,
		})
		return
	}

	ctx := r.Context()
	allNodes, err := h.Store.ListNodes(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	for _, n := range allNodes {
		if len(nodes) < searchGroupLimit && matchesQuery(q, n.Name, n.Host) {
			nodes = append(nodes, searchNodeHit{ID: n.ID, Name: n.Name, Type: n.Type})
		}
		if len(containers) < searchGroupLimit {
			ctrs, _ := h.Store.ListContainersByNode(ctx, n.ID)
			for _, c := range ctrs {
				if len(containers) >= searchGroupLimit {
					break
				}
				if matchesQuery(q, c.Name, c.Image, c.ComposeProject) {
					containers = append(containers, searchContainerHit{
						ID: c.ID, Name: c.Name, NodeID: c.NodeID, Image: c.Image, Status: c.Status,
					})
				}
			}
		}
		if len(vms) < searchGroupLimit {
			nodeVMs, _ := h.Store.ListVMsByNode(ctx, n.ID)
			for _, v := range nodeVMs {
				if len(vms) >= searchGroupLimit {
					break
				}
				if matchesQuery(q, v.Name) {
					vms = append(vms, searchVMHit{ID: v.ID, Name: v.Name, NodeID: v.NodeID})
				}
			}
		}
	}

	if user, ok := middleware.UserFromContext(ctx); ok {
		bms, _ := h.Store.ListBookmarksByUser(ctx, user.ID)
		for _, b := range bms {
			if len(bookmarks) >= searchGroupLimit {
				break
			}
			if matchesQuery(q, b.Label) {
				bookmarks = append(bookmarks, searchBookmarkHit{
					ID: b.ID, Label: b.Label, ResourceType: b.ResourceType, ResourceRef: b.ResourceRef,
				})
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"nodes": nodes, "containers": containers, "vms": vms, "bookmarks": bookmarks,
	})
}
