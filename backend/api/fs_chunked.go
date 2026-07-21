package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/KAE-Labs/stratum/backend/activity"
	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/fs"
)

// uploadChunkMax bounds a single resumable chunk (Feature F10 uses 4MB chunks;
// allow margin). The request body is hard-capped to this.
const uploadChunkMax = 8 << 20

// FSUploadStatus reports how many bytes of a resumable upload already landed, so
// the client can resume after a dropped connection.
func (h *Handlers) FSUploadStatus(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	received, err := h.Files.UploadStatus(r.Context(), chi.URLParam(r, "id"), r.URL.Query().Get("path"))
	if err != nil {
		writeFSError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"received": received})
}

// FSUploadChunk appends one chunk at ?offset= to the resumable temp file. Not
// individually audited (high volume) — the completed upload is audited in
// FSUploadFinish. On an offset mismatch it returns 409 with the true received
// count so the client can re-sync.
func (h *Handlers) FSUploadChunk(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	q := r.URL.Query()
	offset, err := strconv.ParseInt(q.Get("offset"), 10, 64)
	if err != nil || offset < 0 {
		writeError(w, http.StatusBadRequest, "invalid_offset")
		return
	}
	body := http.MaxBytesReader(w, r.Body, uploadChunkMax+1)
	received, err := h.Files.UploadChunk(r.Context(), chi.URLParam(r, "id"), q.Get("path"), offset, body, uploadChunkMax)
	if errors.Is(err, fs.ErrOffsetMismatch) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "offset_mismatch", "received": received})
		return
	}
	if err != nil {
		writeFSError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"received": received})
}

// FSUploadFinish renames the completed partial onto the target. This is the
// audited "file uploaded" event (filename, size, source IP, user). Requires
// ?overwrite=true to replace an existing file.
func (h *Handlers) FSUploadFinish(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	q := r.URL.Query()
	target := q.Get("path")
	size, err := h.Files.UploadFinish(r.Context(), chi.URLParam(r, "id"), target, q.Get("overwrite") == "true")
	if errors.Is(err, fs.ErrExists) {
		writeError(w, http.StatusConflict, "file_exists")
		return
	}
	if err != nil {
		writeFSError(w, err)
		return
	}
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionFSUpload
		e.TargetType = ptr(activity.TargetFile)
		e.TargetID = &target
		e.Detail = map[string]string{"bytes": strconv.FormatInt(size, 10), "resumable": "true"}
	}
	writeJSON(w, http.StatusCreated, map[string]any{"path": target, "bytes": size})
}

// FSUploadCancel discards an in-progress resumable upload's temp file.
func (h *Handlers) FSUploadCancel(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if err := h.Files.UploadCancel(r.Context(), chi.URLParam(r, "id"), r.URL.Query().Get("path")); err != nil {
		// A missing temp file is not an error from the client's perspective.
		if !errors.Is(err, db.ErrNotFound) {
			writeFSError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
