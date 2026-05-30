package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

const nodeColumns = `id, name, type, host, port, auth_method, os_type, capabilities_json,
	credentials_encrypted, credentials_version, ssh_host_key, proxmox_endpoint,
	proxmox_tls_insecure, docker_endpoint, linked_vmid, status, last_error, last_seen, created_at, updated_at`

func (s *Store) CreateNode(ctx context.Context, n appdb.Node) error {
	now := time.Now()
	if n.CreatedAt.IsZero() {
		n.CreatedAt = now
	}
	if n.UpdatedAt.IsZero() {
		n.UpdatedAt = now
	}
	if n.CredentialsVersion == 0 {
		n.CredentialsVersion = 1
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO nodes (`+nodeColumns+`)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.Name, n.Type, n.Host, n.Port, n.AuthMethod, nullableEmpty(n.OSType),
		n.CapabilitiesJSON, n.CredentialsEncrypted, n.CredentialsVersion,
		nullableEmpty(n.SSHHostKey), nullableEmpty(n.ProxmoxEndpoint), boolToInt(n.ProxmoxTLSInsecure),
		nullableEmpty(n.DockerEndpoint), nullableInt(n.LinkedVMID), n.Status, nullableEmpty(n.LastError),
		nullTSText(n.LastSeen), tsText(n.CreatedAt), tsText(n.UpdatedAt))
	if err != nil {
		return fmt.Errorf("sqlite: create node: %w", err)
	}
	return nil
}

func (s *Store) GetNode(ctx context.Context, id string) (appdb.Node, error) {
	return s.scanNode(s.db.QueryRowContext(ctx, `SELECT `+nodeColumns+` FROM nodes WHERE id = ?`, id))
}

func (s *Store) ListNodes(ctx context.Context) ([]appdb.Node, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+nodeColumns+` FROM nodes ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list nodes: %w", err)
	}
	defer rows.Close()
	var out []appdb.Node
	for rows.Next() {
		n, err := scanNodeRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) UpdateNode(ctx context.Context, n appdb.Node) error {
	n.UpdatedAt = time.Now()
	res, err := s.db.ExecContext(ctx,
		`UPDATE nodes SET name=?, type=?, host=?, port=?, auth_method=?, os_type=?,
		   capabilities_json=?, credentials_encrypted=?, credentials_version=?, ssh_host_key=?,
		   proxmox_endpoint=?, proxmox_tls_insecure=?, docker_endpoint=?, linked_vmid=?, status=?, last_error=?,
		   last_seen=?, updated_at=?
		 WHERE id=?`,
		n.Name, n.Type, n.Host, n.Port, n.AuthMethod, nullableEmpty(n.OSType),
		n.CapabilitiesJSON, n.CredentialsEncrypted, n.CredentialsVersion, nullableEmpty(n.SSHHostKey),
		nullableEmpty(n.ProxmoxEndpoint), boolToInt(n.ProxmoxTLSInsecure), nullableEmpty(n.DockerEndpoint),
		nullableInt(n.LinkedVMID), n.Status, nullableEmpty(n.LastError), nullTSText(n.LastSeen), tsText(n.UpdatedAt), n.ID)
	if err != nil {
		return fmt.Errorf("sqlite: update node: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteNode(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM nodes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete node: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func (s *Store) scanNode(row *sql.Row) (appdb.Node, error) {
	n, err := scanNodeRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.Node{}, appdb.ErrNotFound
	}
	return n, err
}

func scanNodeRow(row rowScanner) (appdb.Node, error) {
	var n appdb.Node
	var osType, sshHostKey, proxmoxEndpoint, dockerEndpoint, lastError sql.NullString
	var lastSeen, createdAt, updatedAt sql.NullString
	var proxmoxInsecure int
	var linkedVMID sql.NullInt64
	err := row.Scan(
		&n.ID, &n.Name, &n.Type, &n.Host, &n.Port, &n.AuthMethod, &osType, &n.CapabilitiesJSON,
		&n.CredentialsEncrypted, &n.CredentialsVersion, &sshHostKey, &proxmoxEndpoint,
		&proxmoxInsecure, &dockerEndpoint, &linkedVMID, &n.Status, &lastError, &lastSeen, &createdAt, &updatedAt)
	if err != nil {
		return appdb.Node{}, err
	}
	n.OSType = osType.String
	n.SSHHostKey = sshHostKey.String
	n.ProxmoxEndpoint = proxmoxEndpoint.String
	n.DockerEndpoint = dockerEndpoint.String
	n.LastError = lastError.String
	n.ProxmoxTLSInsecure = proxmoxInsecure != 0
	if linkedVMID.Valid {
		v := int(linkedVMID.Int64)
		n.LinkedVMID = &v
	}
	if n.LastSeen, err = scanNullTS(lastSeen); err != nil {
		return appdb.Node{}, err
	}
	if n.CreatedAt, err = scanTS(createdAt); err != nil {
		return appdb.Node{}, err
	}
	if n.UpdatedAt, err = scanTS(updatedAt); err != nil {
		return appdb.Node{}, err
	}
	return n, nil
}
