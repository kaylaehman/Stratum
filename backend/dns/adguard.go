package dns

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

func init() { Register(&AdGuard{HTTP: http.DefaultClient}) }

// AdGuard reads DNS rewrites from AdGuard Home's control API. Auth is the
// admin session token supplied via Conn.Token (sent as a Bearer header; AdGuard
// also accepts a basic-auth cookie, but token keeps it simple and testable).
type AdGuard struct {
	HTTP *http.Client
}

func (a *AdGuard) Name() string            { return "adguardhome" }
func (a *AdGuard) ImagePatterns() []string { return []string{"adguard/adguardhome", "adguardhome"} }
func (a *AdGuard) Capabilities() Capabilities {
	return Capabilities{List: true} // create/update/delete are additive follow-ons
}

type adguardRewrite struct {
	Domain string `json:"domain"`
	Answer string `json:"answer"`
}

func (a *AdGuard) ListRecords(ctx context.Context, conn Conn) ([]Record, error) {
	if conn.Endpoint == "" {
		return nil, fmt.Errorf("adguard: api endpoint not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimRight(conn.Endpoint, "/")+"/control/rewrite/list", nil)
	if err != nil {
		return nil, err
	}
	if conn.Token != "" {
		req.Header.Set("Authorization", "Bearer "+conn.Token)
	}
	resp, err := a.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("adguard: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("adguard: status %d", resp.StatusCode)
	}
	var rewrites []adguardRewrite
	if err := json.Unmarshal(raw, &rewrites); err != nil {
		return nil, fmt.Errorf("adguard: decode: %w", err)
	}
	records := make([]Record, 0, len(rewrites))
	for i, rw := range rewrites {
		records = append(records, Record{
			ID:          fmt.Sprintf("%s|%s", rw.Domain, rw.Answer),
			AdapterType: a.Name(),
			Type:        recordTypeForAnswer(rw.Answer),
			Name:        rw.Domain,
			Value:       rw.Answer,
		})
		_ = i
	}
	return records, nil
}

func (a *AdGuard) CreateRecord(context.Context, Conn, Record) (Record, error) {
	return Record{}, ErrUnsupported
}
func (a *AdGuard) UpdateRecord(context.Context, Conn, string, Record) error { return ErrUnsupported }
func (a *AdGuard) DeleteRecord(context.Context, Conn, string) error         { return ErrUnsupported }

// recordTypeForAnswer infers a record type from a rewrite answer value.
func recordTypeForAnswer(answer string) string {
	ip := net.ParseIP(answer)
	switch {
	case ip == nil:
		return "CNAME"
	case ip.To4() != nil:
		return "A"
	default:
		return "AAAA"
	}
}
