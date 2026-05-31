package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

func marshalStrings(s []string) string {
	if s == nil {
		s = []string{}
	}
	b, _ := json.Marshal(s)
	return string(b)
}

func unmarshalStrings(s string) []string {
	var out []string
	if s != "" {
		_ = json.Unmarshal([]byte(s), &out)
	}
	return out
}

const csColumns = `container_id, node_id, privileged, cap_add_all, dangerous_caps, seccomp_unconfined,
	apparmor_unconfined, devices, userns_host, pid_host, net_host, runs_as_root, run_uid, scanned_at`

func (s *Store) UpsertContainerSecurity(ctx context.Context, r appdb.ContainerSecurityRow) error {
	if r.ScannedAt.IsZero() {
		r.ScannedAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO container_security (`+csColumns+`)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(container_id) DO UPDATE SET
		   node_id=excluded.node_id, privileged=excluded.privileged, cap_add_all=excluded.cap_add_all,
		   dangerous_caps=excluded.dangerous_caps, seccomp_unconfined=excluded.seccomp_unconfined,
		   apparmor_unconfined=excluded.apparmor_unconfined, devices=excluded.devices,
		   userns_host=excluded.userns_host, pid_host=excluded.pid_host, net_host=excluded.net_host,
		   runs_as_root=excluded.runs_as_root, run_uid=excluded.run_uid, scanned_at=excluded.scanned_at`,
		r.ContainerID, r.NodeID, boolToInt(r.Privileged), boolToInt(r.CapAddAll), marshalStrings(r.DangerousCaps),
		boolToInt(r.SeccompUnconfined), boolToInt(r.ApparmorUnconfined), marshalStrings(r.Devices),
		boolToInt(r.UsernsHost), boolToInt(r.PidHost), boolToInt(r.NetHost), boolToInt(r.RunsAsRoot), r.RunUID, tsText(r.ScannedAt))
	if err != nil {
		return fmt.Errorf("sqlite: upsert container_security: %w", err)
	}
	return nil
}

func scanCS(sc rowScanner) (appdb.ContainerSecurityRow, error) {
	var r appdb.ContainerSecurityRow
	var caps, devices, scannedAt string
	var priv, capAll, seccomp, apparmor, userns, pid, net, root int
	err := sc.Scan(&r.ContainerID, &r.NodeID, &priv, &capAll, &caps, &seccomp, &apparmor, &devices, &userns, &pid, &net, &root, &r.RunUID, &scannedAt)
	if err != nil {
		return appdb.ContainerSecurityRow{}, err
	}
	r.Privileged, r.CapAddAll = priv != 0, capAll != 0
	r.SeccompUnconfined, r.ApparmorUnconfined = seccomp != 0, apparmor != 0
	r.UsernsHost, r.PidHost, r.NetHost, r.RunsAsRoot = userns != 0, pid != 0, net != 0, root != 0
	r.DangerousCaps = unmarshalStrings(caps)
	r.Devices = unmarshalStrings(devices)
	r.ScannedAt, _ = parseTS(scannedAt)
	return r, nil
}

func (s *Store) GetContainerSecurity(ctx context.Context, containerID string) (appdb.ContainerSecurityRow, error) {
	r, err := scanCS(s.db.QueryRowContext(ctx, `SELECT `+csColumns+` FROM container_security WHERE container_id = ?`, containerID))
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.ContainerSecurityRow{}, appdb.ErrNotFound
	}
	if err != nil {
		return appdb.ContainerSecurityRow{}, fmt.Errorf("sqlite: get container_security: %w", err)
	}
	return r, nil
}

func (s *Store) ListContainerSecurity(ctx context.Context) ([]appdb.ContainerSecurityRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+csColumns+` FROM container_security`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list container_security: %w", err)
	}
	defer rows.Close()
	var out []appdb.ContainerSecurityRow
	for rows.Next() {
		r, err := scanCS(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

const peColumns = `id, node_id, container_id, host_ip, host_port, container_port, protocol, interface_class, is_new, notified_at, first_seen, last_seen`

func (s *Store) SetPortExposures(ctx context.Context, containerID string, rows []appdb.PortExposureRow) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM port_exposures WHERE container_id = ?`, containerID); err != nil {
		return fmt.Errorf("sqlite: clear port_exposures: %w", err)
	}
	for _, p := range rows {
		if p.FirstSeen.IsZero() {
			p.FirstSeen = time.Now()
		}
		if p.LastSeen.IsZero() {
			p.LastSeen = time.Now()
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO port_exposures (`+peColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			p.ID, p.NodeID, p.ContainerID, p.HostIP, p.HostPort, p.ContainerPort, p.Protocol, p.InterfaceClass,
			boolToInt(p.IsNew), nullTSText(p.NotifiedAt), tsText(p.FirstSeen), tsText(p.LastSeen)); err != nil {
			return fmt.Errorf("sqlite: insert port_exposure: %w", err)
		}
	}
	return tx.Commit()
}

func scanPE(sc rowScanner) (appdb.PortExposureRow, error) {
	var p appdb.PortExposureRow
	var isNew int
	var notifiedAt, firstSeen, lastSeen sql.NullString
	err := sc.Scan(&p.ID, &p.NodeID, &p.ContainerID, &p.HostIP, &p.HostPort, &p.ContainerPort, &p.Protocol, &p.InterfaceClass, &isNew, &notifiedAt, &firstSeen, &lastSeen)
	if err != nil {
		return appdb.PortExposureRow{}, err
	}
	p.IsNew = isNew != 0
	if p.NotifiedAt, err = scanNullTS(notifiedAt); err != nil {
		return appdb.PortExposureRow{}, err
	}
	p.FirstSeen, _ = scanTS(firstSeen)
	p.LastSeen, _ = scanTS(lastSeen)
	return p, nil
}

func (s *Store) ListPortExposuresByContainer(ctx context.Context, containerID string) ([]appdb.PortExposureRow, error) {
	return s.queryPEs(ctx, `SELECT `+peColumns+` FROM port_exposures WHERE container_id = ?`, containerID)
}

func (s *Store) ListPortExposuresByNode(ctx context.Context, nodeID string) ([]appdb.PortExposureRow, error) {
	return s.queryPEs(ctx, `SELECT `+peColumns+` FROM port_exposures WHERE node_id = ?`, nodeID)
}

func (s *Store) ListAllPortExposures(ctx context.Context) ([]appdb.PortExposureRow, error) {
	return s.queryPEs(ctx, `SELECT `+peColumns+` FROM port_exposures`)
}

func (s *Store) queryPEs(ctx context.Context, query string, args ...any) ([]appdb.PortExposureRow, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: query port_exposures: %w", err)
	}
	defer rows.Close()
	var out []appdb.PortExposureRow
	for rows.Next() {
		p, err := scanPE(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) InsertAck(ctx context.Context, a appdb.SecurityAck) error {
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO security_acknowledgements (id, node_id, container_id, flag_type, flag_key, acknowledged_by, note, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(container_id, flag_type, flag_key) DO UPDATE SET note=excluded.note, acknowledged_by=excluded.acknowledged_by`,
		a.ID, a.NodeID, a.ContainerID, a.FlagType, a.FlagKey, nullStr(a.AcknowledgedBy), nullableEmpty(a.Note), tsText(a.CreatedAt))
	if err != nil {
		return fmt.Errorf("sqlite: insert ack: %w", err)
	}
	return nil
}

func (s *Store) DeleteAck(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM security_acknowledgements WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete ack: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

func (s *Store) ListAcks(ctx context.Context) ([]appdb.SecurityAck, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, node_id, container_id, flag_type, flag_key, acknowledged_by, note, created_at FROM security_acknowledgements`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list acks: %w", err)
	}
	defer rows.Close()
	var out []appdb.SecurityAck
	for rows.Next() {
		var a appdb.SecurityAck
		var by, note, createdAt sql.NullString
		if err := rows.Scan(&a.ID, &a.NodeID, &a.ContainerID, &a.FlagType, &a.FlagKey, &by, &note, &createdAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan ack: %w", err)
		}
		a.AcknowledgedBy = ptrFromNull(by)
		a.Note = note.String
		a.CreatedAt, _ = parseTS(createdAt.String)
		out = append(out, a)
	}
	return out, rows.Err()
}
