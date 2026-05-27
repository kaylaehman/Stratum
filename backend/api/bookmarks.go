package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/middleware"
)

type bookmarkView struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	ResourceType string `json:"resource_type"`
	ResourceRef  string `json:"resource_ref"`
	GroupName    string `json:"group_name"`
	OrderIndex   int    `json:"order_index"`
}

func toBookmarkView(b db.Bookmark) bookmarkView {
	return bookmarkView{
		ID: b.ID, Label: b.Label, ResourceType: b.ResourceType,
		ResourceRef: b.ResourceRef, GroupName: b.GroupName, OrderIndex: b.OrderIndex,
	}
}

// ListBookmarks returns the calling user's bookmarks, ordered.
func (h *Handlers) ListBookmarks(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.UserFromContext(r.Context())
	rows, err := h.Store.ListBookmarksByUser(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	views := make([]bookmarkView, len(rows))
	for i, b := range rows {
		views[i] = toBookmarkView(b)
	}
	writeJSON(w, http.StatusOK, map[string]any{"bookmarks": views})
}

type createBookmarkBody struct {
	Label        string `json:"label"`
	ResourceType string `json:"resource_type"`
	ResourceRef  string `json:"resource_ref"`
	GroupName    string `json:"group_name"`
}

// CreateBookmark adds a bookmark for the calling user.
func (h *Handlers) CreateBookmark(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.UserFromContext(r.Context())
	var body createBookmarkBody
	if err := decodeJSON(r, &body); err != nil || body.Label == "" || body.ResourceType == "" || body.ResourceRef == "" {
		writeError(w, http.StatusBadRequest, "label_type_ref_required")
		return
	}
	b := db.Bookmark{
		ID: uuid.NewString(), UserID: user.ID, Label: body.Label,
		ResourceType: body.ResourceType, ResourceRef: body.ResourceRef, GroupName: body.GroupName,
	}
	if err := h.Store.CreateBookmark(r.Context(), b); err != nil {
		writeError(w, http.StatusInternalServerError, "create_failed")
		return
	}
	writeJSON(w, http.StatusCreated, toBookmarkView(b))
}

// DeleteBookmark removes one of the calling user's bookmarks.
func (h *Handlers) DeleteBookmark(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.Store.DeleteBookmark(r.Context(), id, user.ID); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type reorderBody struct {
	IDs []string `json:"ids"`
}

// ReorderBookmarks sets the order of the calling user's bookmarks.
func (h *Handlers) ReorderBookmarks(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.UserFromContext(r.Context())
	var body reorderBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "ids_required")
		return
	}
	if err := h.Store.SetBookmarkOrder(r.Context(), user.ID, body.IDs); err != nil {
		writeError(w, http.StatusInternalServerError, "reorder_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
