package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func init() { Register(&NPM{HTTP: http.DefaultClient}) }

// NPM (Nginx Proxy Manager) reads proxy hosts from its REST API. A bearer token
// (obtained out-of-band and stored in the secrets vault) is required; the
// service supplies it via Conn.Token.
type NPM struct {
	HTTP *http.Client
}

func (n *NPM) Name() string            { return "nginx-proxy-manager" }
func (n *NPM) ImagePatterns() []string { return []string{"jc21/nginx-proxy-manager", "nginx-proxy-manager"} }
func (n *NPM) Capabilities() Capabilities {
	return Capabilities{List: true} // create/update/delete are additive follow-ons
}

type npmProxyHost struct {
	ID            int      `json:"id"`
	DomainNames   []string `json:"domain_names"`
	ForwardHost   string   `json:"forward_host"`
	ForwardPort   int      `json:"forward_port"`
	ForwardScheme string   `json:"forward_scheme"`
	SSLForced     bool     `json:"ssl_forced"`
	CertificateID int      `json:"certificate_id"`
}

func (n *NPM) ListRules(ctx context.Context, conn Conn) ([]Rule, error) {
	if conn.Endpoint == "" {
		return nil, fmt.Errorf("npm: api endpoint not configured")
	}
	if conn.Token == "" {
		return nil, fmt.Errorf("npm: api token not configured (store it in the secrets vault)")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimRight(conn.Endpoint, "/")+"/api/nginx/proxy-hosts", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+conn.Token)
	resp, err := n.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("npm: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("npm: status %d", resp.StatusCode)
	}
	var hosts []npmProxyHost
	if err := json.Unmarshal(raw, &hosts); err != nil {
		return nil, fmt.Errorf("npm: decode: %w", err)
	}
	rules := make([]Rule, 0, len(hosts))
	for _, h := range hosts {
		host := ""
		if len(h.DomainNames) > 0 {
			host = h.DomainNames[0]
		}
		scheme := h.ForwardScheme
		if scheme == "" {
			scheme = "http"
		}
		rules = append(rules, Rule{
			ID:          strconv.Itoa(h.ID),
			AdapterType: n.Name(),
			SourceHost:  host,
			TargetURL:   fmt.Sprintf("%s://%s:%d", scheme, h.ForwardHost, h.ForwardPort),
			SSLEnabled:  h.SSLForced,
			CertID:      certIDString(h.CertificateID),
		})
	}
	return rules, nil
}

func (n *NPM) CreateRule(context.Context, Conn, Rule) (Rule, error) { return Rule{}, ErrUnsupported }
func (n *NPM) UpdateRule(context.Context, Conn, string, Rule) error { return ErrUnsupported }
func (n *NPM) DeleteRule(context.Context, Conn, string) error       { return ErrUnsupported }

func certIDString(id int) string {
	if id <= 0 {
		return ""
	}
	return strconv.Itoa(id)
}
