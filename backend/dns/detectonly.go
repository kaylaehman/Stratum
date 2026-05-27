package dns

import "context"

func init() {
	Register(&detectOnly{name: "pihole", patterns: []string{"pihole/pihole", "pihole"}})
	Register(&detectOnly{name: "technitium", patterns: []string{"technitium/dns-server", "technitium"}})
	Register(&detectOnly{name: "coredns", patterns: []string{"coredns/coredns", "coredns"}})
}

// detectOnly recognises a DNS tool but does not yet manage its records via API
// (Pi-hole/Technitium need session auth flows; CoreDNS is Corefile-based).
// These surface "detected" with no rule set; a richer adapter can replace the
// registration without touching the API/UI/model.
type detectOnly struct {
	name     string
	patterns []string
}

func (d *detectOnly) Name() string               { return d.name }
func (d *detectOnly) ImagePatterns() []string    { return d.patterns }
func (d *detectOnly) Capabilities() Capabilities { return Capabilities{} }
func (d *detectOnly) ListRecords(context.Context, Conn) ([]Record, error) {
	return nil, ErrUnsupported
}
func (d *detectOnly) CreateRecord(context.Context, Conn, Record) (Record, error) {
	return Record{}, ErrUnsupported
}
func (d *detectOnly) UpdateRecord(context.Context, Conn, string, Record) error { return ErrUnsupported }
func (d *detectOnly) DeleteRecord(context.Context, Conn, string) error         { return ErrUnsupported }
