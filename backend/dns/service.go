package dns

import (
	"context"
	"net/http"
	"time"

	"github.com/KAE-Labs/stratum/backend/crypto"
	"github.com/KAE-Labs/stratum/backend/db"
)

const httpTimeout = 15 * time.Second

// Service detects a node's DNS tool from its container inventory and lists its
// records through the matching adapter when an admin endpoint is configured.
type Service struct {
	store  db.Store
	cipher *crypto.Cipher
}

// New wires the store + secret cipher and points API adapters at a bounded client.
func New(store db.Store, cipher *crypto.Cipher) *Service {
	hc := &http.Client{Timeout: httpTimeout}
	for _, a := range Adapters() {
		if ag, ok := a.(*AdGuard); ok {
			ag.HTTP = hc
		}
	}
	return &Service{store: store, cipher: cipher}
}

// Status is the API view for a node's DNS tool.
type Status struct {
	Detected     string       `json:"detected"`
	Capabilities Capabilities `json:"capabilities"`
	Configured   bool         `json:"configured"`
	Endpoint     string       `json:"endpoint,omitempty"`
	HasToken     bool         `json:"has_token"`
	Records      []Record     `json:"records"`
	RecordError  string       `json:"record_error,omitempty"`
	Supported    []ToolInfo   `json:"supported"`
}

func (s *Service) detect(ctx context.Context, nodeID string) (Adapter, error) {
	containers, err := s.store.ListContainersByNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	images := make([]string, 0, len(containers))
	for _, c := range containers {
		images = append(images, c.Image)
	}
	return DetectByImages(images), nil
}

func (s *Service) conn(ctx context.Context, nodeID string) (Conn, bool) {
	cfg, err := s.store.GetDNSConfig(ctx, nodeID)
	if err != nil {
		return Conn{}, false
	}
	c := Conn{Endpoint: cfg.Endpoint}
	if len(cfg.TokenEncrypted) > 0 {
		if pt, derr := s.cipher.Open(cfg.TokenEncrypted); derr == nil {
			c.Token = string(pt)
		}
	}
	return c, cfg.Endpoint != ""
}

// Status detects the node's DNS tool and lists records when possible.
func (s *Service) Status(ctx context.Context, nodeID string) (Status, error) {
	st := Status{Supported: SupportedTools(), Records: []Record{}}
	adapter, err := s.detect(ctx, nodeID)
	if err != nil {
		return st, err
	}
	if adapter == nil {
		return st, nil
	}
	st.Detected = adapter.Name()
	st.Capabilities = adapter.Capabilities()

	conn, configured := s.conn(ctx, nodeID)
	st.Configured = configured
	st.Endpoint = conn.Endpoint
	st.HasToken = conn.Token != ""

	if adapter.Capabilities().List && configured {
		recs, lerr := adapter.ListRecords(ctx, conn)
		if lerr != nil {
			st.RecordError = lerr.Error()
		} else {
			st.Records = recs
		}
	} else if adapter.Capabilities().List && !configured {
		st.RecordError = "admin endpoint not configured"
	}
	return st, nil
}

// SetConfig stores a node's DNS admin endpoint + optional token (sealed).
func (s *Service) SetConfig(ctx context.Context, nodeID, endpoint string, token *string) error {
	existing, _ := s.store.GetDNSConfig(ctx, nodeID)
	cfg := db.DNSConfig{NodeID: nodeID, Endpoint: endpoint, TokenEncrypted: existing.TokenEncrypted}
	if token != nil {
		if *token == "" {
			cfg.TokenEncrypted = nil
		} else {
			sealed, err := s.cipher.Seal([]byte(*token))
			if err != nil {
				return err
			}
			cfg.TokenEncrypted = sealed
		}
	}
	return s.store.UpsertDNSConfig(ctx, cfg)
}
