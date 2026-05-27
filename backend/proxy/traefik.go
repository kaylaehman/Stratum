package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func init() { Register(&Traefik{HTTP: http.DefaultClient}) }

// Traefik reads routes from the Traefik HTTP API (read-only: Traefik's dynamic
// config is owned by its providers, so Stratum surfaces but does not edit it).
type Traefik struct {
	HTTP *http.Client
}

func (t *Traefik) Name() string            { return "traefik" }
func (t *Traefik) ImagePatterns() []string { return []string{"traefik"} }
func (t *Traefik) Capabilities() Capabilities {
	return Capabilities{List: true} // dynamic config is provider-owned; read-only
}

type traefikRouter struct {
	Name    string `json:"name"`
	Rule    string `json:"rule"`
	Service string `json:"service"`
	TLS     *struct {
		CertResolver string `json:"certResolver"`
	} `json:"tls"`
}

func (t *Traefik) ListRules(ctx context.Context, conn Conn) ([]Rule, error) {
	if conn.Endpoint == "" {
		return nil, fmt.Errorf("traefik: admin api endpoint not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(conn.Endpoint, "/")+"/api/http/routers", nil)
	if err != nil {
		return nil, err
	}
	resp, err := t.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("traefik: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("traefik: status %d", resp.StatusCode)
	}
	var routers []traefikRouter
	if err := json.Unmarshal(raw, &routers); err != nil {
		return nil, fmt.Errorf("traefik: decode: %w", err)
	}
	rules := make([]Rule, 0, len(routers))
	for _, r := range routers {
		rules = append(rules, Rule{
			ID:          r.Name,
			AdapterType: t.Name(),
			SourceHost:  hostFromTraefikRule(r.Rule),
			TargetURL:   r.Service,
			SSLEnabled:  r.TLS != nil,
		})
	}
	return rules, nil
}

func (t *Traefik) CreateRule(context.Context, Conn, Rule) (Rule, error) {
	return Rule{}, ErrUnsupported
}
func (t *Traefik) UpdateRule(context.Context, Conn, string, Rule) error { return ErrUnsupported }
func (t *Traefik) DeleteRule(context.Context, Conn, string) error       { return ErrUnsupported }

// hostFromTraefikRule extracts the host from a Traefik rule like
// "Host(`app.example.com`)" or "Host(`a`) && PathPrefix(`/x`)".
func hostFromTraefikRule(rule string) string {
	const marker = "Host(`"
	i := strings.Index(rule, marker)
	if i < 0 {
		return ""
	}
	rest := rule[i+len(marker):]
	if j := strings.Index(rest, "`"); j >= 0 {
		return rest[:j]
	}
	return ""
}
