package db

import "time"

// ConfigVersion is one snapshot of a tracked config file on a node.
// Content is the verbatim file text (capped at 1 MB before write).
// Hash is the hex-encoded SHA-256 of Content.
type ConfigVersion struct {
	ID        string    `json:"id"`
	NodeID    string    `json:"node_id"`
	Path      string    `json:"path"`
	Content   string    `json:"content,omitempty"` // omitted in list responses
	Hash      string    `json:"hash"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
}
