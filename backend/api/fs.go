package api

import (
	"errors"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/fs"
	"github.com/kaylaehman/stratum/backend/middleware"
)

// writeFSError maps service errors to HTTP statuses.
func writeFSError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, fs.ErrInvalidPath):
		writeError(w, http.StatusBadRequest, "invalid_path")
	case errors.Is(err, fs.ErrDenied):
		writeError(w, http.StatusForbidden, "path_denied")
	case errors.Is(err, fs.ErrStale):
		writeError(w, http.StatusPreconditionFailed, "stale")
	case errors.Is(err, fs.ErrTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "too_large")
	case errors.Is(err, db.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found")
	default:
		// Unreachable node / SFTP error — best-effort 502.
		writeError(w, http.StatusBadGateway, "fs_error")
	}
}

// FSList lists a directory.
func (h *Handlers) FSList(w http.ResponseWriter, r *http.Request) {
	entries, truncated, err := h.Files.List(r.Context(), chi.URLParam(r, "id"), r.URL.Query().Get("path"))
	if err != nil {
		writeFSError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries, "truncated": truncated})
}

// FSReadFile returns a file's inline preview (<=5MB) or a tooLarge flag, with a
// Last-Modified header for lost-update protection on PUT.
func (h *Handlers) FSReadFile(w http.ResponseWriter, r *http.Request) {
	content, tooLarge, modTime, err := h.Files.Preview(r.Context(), chi.URLParam(r, "id"), r.URL.Query().Get("path"))
	if err != nil {
		writeFSError(w, err)
		return
	}
	if !modTime.IsZero() {
		w.Header().Set("Last-Modified", modTime.UTC().Format(http.TimeFormat))
	}
	if tooLarge {
		writeJSON(w, http.StatusOK, map[string]any{"too_large": true})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"too_large": false, "content": string(content)})
}

// FSWriteFile replaces a file's content. Honors If-Unmodified-Since -> 412.
func (h *Handlers) FSWriteFile(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	filePath := r.URL.Query().Get("path")
	limit := h.Files.UploadMax()
	body, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_body")
		return
	}
	if int64(len(body)) > limit {
		writeError(w, http.StatusRequestEntityTooLarge, "too_large")
		return
	}
	var ifUnmod *time.Time
	if v := r.Header.Get("If-Unmodified-Since"); v != "" {
		if t, perr := time.Parse(http.TimeFormat, v); perr == nil {
			ifUnmod = &t
		}
	}
	if err := h.Files.Write(r.Context(), chi.URLParam(r, "id"), filePath, body, ifUnmod); err != nil {
		writeFSError(w, err)
		return
	}
	enrichFSActivity(r, "fs.write", filePath)
	w.WriteHeader(http.StatusNoContent)
}

// FSDownload streams a file.
func (h *Handlers) FSDownload(w http.ResponseWriter, r *http.Request) {
	rc, err := h.Files.Download(r.Context(), chi.URLParam(r, "id"), r.URL.Query().Get("path"))
	if err != nil {
		writeFSError(w, err)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = io.Copy(w, rc)
}

// FSUpload streams an uploaded file to the target directory (direct-Append audit
// only on success; suppresses the middleware's deferred write).
func (h *Handlers) FSUpload(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	// Suppress the activity middleware's deferred write; we Append directly.
	entry := activity.FromContext(r.Context())
	if entry != nil {
		entry.Suppressed = true
	}

	destDir := r.URL.Query().Get("path")
	mr, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "expected_multipart")
		return
	}
	part, err := mr.NextPart()
	if err != nil {
		writeError(w, http.StatusBadRequest, "no_file_part")
		return
	}
	defer part.Close()

	// Strip any directory components from the client-supplied filename to prevent
	// path traversal (e.g. "../../etc/passwd"); only the base name is used.
	name := path.Base(part.FileName())
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		writeError(w, http.StatusBadRequest, "bad_filename")
		return
	}
	target := path.Join(destDir, name)
	written, err := h.Files.Upload(r.Context(), chi.URLParam(r, "id"), target, part)
	result := activity.ResultSuccess
	if err != nil {
		result = activity.ResultError
	}
	if uid, ok := middleware.UserFromContext(r.Context()); ok {
		_ = h.Activity.Append(r.Context(), activity.Entry{
			UserID: &uid.ID, Action: "fs.upload", TargetType: ptr("file"), TargetID: &target,
			Detail: map[string]any{"bytes": written}, Result: result,
		})
	}
	if err != nil {
		writeFSError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"path": target, "bytes": written})
}

type fsMkdirBody struct {
	Path string `json:"path"`
}

// FSMkdir creates a directory.
func (h *Handlers) FSMkdir(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	var body fsMkdirBody
	if err := decodeJSON(r, &body); err != nil || body.Path == "" {
		writeError(w, http.StatusBadRequest, "path_required")
		return
	}
	if err := h.Files.Mkdir(r.Context(), chi.URLParam(r, "id"), body.Path); err != nil {
		writeFSError(w, err)
		return
	}
	enrichFSActivity(r, "fs.mkdir", body.Path)
	w.WriteHeader(http.StatusCreated)
}

type fsRenameBody struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// FSRename moves/renames a path.
func (h *Handlers) FSRename(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	var body fsRenameBody
	if err := decodeJSON(r, &body); err != nil || body.From == "" || body.To == "" {
		writeError(w, http.StatusBadRequest, "from_and_to_required")
		return
	}
	if err := h.Files.Rename(r.Context(), chi.URLParam(r, "id"), body.From, body.To); err != nil {
		writeFSError(w, err)
		return
	}
	enrichFSActivity(r, "fs.rename", body.From)
	w.WriteHeader(http.StatusNoContent)
}

// FSDelete removes a path. Requires confirm=yes; directories require recursive=true.
func (h *Handlers) FSDelete(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	q := r.URL.Query()
	if q.Get("confirm") != "yes" {
		writeError(w, http.StatusBadRequest, "confirmation_required")
		return
	}
	path := q.Get("path")
	recursive := q.Get("recursive") == "true"
	if err := h.Files.Remove(r.Context(), chi.URLParam(r, "id"), path, recursive); err != nil {
		writeFSError(w, err)
		return
	}
	enrichFSActivity(r, "fs.delete", path)
	w.WriteHeader(http.StatusNoContent)
}

func enrichFSActivity(r *http.Request, action, path string) {
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = action
		e.TargetType = ptr("file")
		p := path
		e.TargetID = &p
	}
}
