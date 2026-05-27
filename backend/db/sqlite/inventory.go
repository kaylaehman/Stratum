package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

const vmColumns = `id, node_id, kind, proxmox_vmid, proxmox_node, name, status, os_type, stale, gone_since, last_seen`

func (s *Store) UpsertVM(ctx context.Context, v appdb.VM) error {
	if v.LastSeen.IsZero() {
		v.LastSeen = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO vms (`+vmColumns+`)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(node_id, proxmox_vmid, kind) DO UPDATE SET
		   proxmox_node=excluded.proxmox_node, name=excluded.name, status=excluded.status,
		   os_type=excluded.os_type, stale=excluded.stale, gone_since=excluded.gone_since,
		   last_seen=excluded.last_seen`,
		v.ID, v.NodeID, v.Kind, v.ProxmoxVMID, v.ProxmoxNode, v.Name, v.Status,
		nullableEmpty(v.OSType), boolToInt(v.Stale), nullTSText(v.GoneSince), tsText(v.LastSeen))
	if err != nil {
		return fmt.Errorf("sqlite: upsert vm: %w", err)
	}
	return nil
}

func (s *Store) ListVMsByNode(ctx context.Context, nodeID string) ([]appdb.VM, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+vmColumns+` FROM vms WHERE node_id = ? ORDER BY proxmox_vmid`, nodeID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list vms: %w", err)
	}
	defer rows.Close()
	var out []appdb.VM
	for rows.Next() {
		var v appdb.VM
		var osType, goneSince, lastSeen sql.NullString
		var stale int
		if err := rows.Scan(&v.ID, &v.NodeID, &v.Kind, &v.ProxmoxVMID, &v.ProxmoxNode, &v.Name, &v.Status, &osType, &stale, &goneSince, &lastSeen); err != nil {
			return nil, fmt.Errorf("sqlite: scan vm: %w", err)
		}
		v.OSType = osType.String
		v.Stale = stale != 0
		if v.GoneSince, err = scanNullTS(goneSince); err != nil {
			return nil, err
		}
		if v.LastSeen, err = scanTS(lastSeen); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) DeleteVM(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM vms WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete vm: %w", err)
	}
	return nil
}

const containerColumns = `id, node_id, docker_id, name, image, image_id, status, compose_project, stale, gone_since, last_seen`

func (s *Store) UpsertContainer(ctx context.Context, c appdb.Container) error {
	if c.LastSeen.IsZero() {
		c.LastSeen = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO containers (`+containerColumns+`)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(node_id, docker_id) DO UPDATE SET
		   name=excluded.name, image=excluded.image, image_id=excluded.image_id,
		   status=excluded.status, compose_project=excluded.compose_project,
		   stale=excluded.stale, gone_since=excluded.gone_since, last_seen=excluded.last_seen`,
		c.ID, c.NodeID, c.DockerID, c.Name, c.Image, nullableEmpty(c.ImageID), c.Status,
		nullableEmpty(c.ComposeProject), boolToInt(c.Stale), nullTSText(c.GoneSince), tsText(c.LastSeen))
	if err != nil {
		return fmt.Errorf("sqlite: upsert container: %w", err)
	}
	return nil
}

func (s *Store) ListContainersByNode(ctx context.Context, nodeID string) ([]appdb.Container, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+containerColumns+` FROM containers WHERE node_id = ? ORDER BY name`, nodeID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list containers: %w", err)
	}
	defer rows.Close()
	var out []appdb.Container
	for rows.Next() {
		var c appdb.Container
		var imageID, composeProject, goneSince, lastSeen sql.NullString
		var stale int
		if err := rows.Scan(&c.ID, &c.NodeID, &c.DockerID, &c.Name, &c.Image, &imageID, &c.Status, &composeProject, &stale, &goneSince, &lastSeen); err != nil {
			return nil, fmt.Errorf("sqlite: scan container: %w", err)
		}
		c.ImageID = imageID.String
		c.ComposeProject = composeProject.String
		c.Stale = stale != 0
		if c.GoneSince, err = scanNullTS(goneSince); err != nil {
			return nil, err
		}
		if c.LastSeen, err = scanTS(lastSeen); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) DeleteContainer(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM containers WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete container: %w", err)
	}
	return nil
}
