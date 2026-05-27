package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

const imageScanColumns = `image_digest, image, scanned_at, critical, high, medium, low, unknown`

func (s *Store) UpsertImageScan(ctx context.Context, r appdb.ImageScanRow) error {
	if r.ScannedAt.IsZero() {
		r.ScannedAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO image_scans (`+imageScanColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(image_digest) DO UPDATE SET image=excluded.image, scanned_at=excluded.scanned_at,
		   critical=excluded.critical, high=excluded.high, medium=excluded.medium, low=excluded.low, unknown=excluded.unknown`,
		r.ImageDigest, r.Image, tsText(r.ScannedAt), r.Critical, r.High, r.Medium, r.Low, r.Unknown)
	if err != nil {
		return fmt.Errorf("sqlite: upsert image scan: %w", err)
	}
	return nil
}

func scanImageScan(sc rowScanner) (appdb.ImageScanRow, error) {
	var r appdb.ImageScanRow
	var scannedAt string
	if err := sc.Scan(&r.ImageDigest, &r.Image, &scannedAt, &r.Critical, &r.High, &r.Medium, &r.Low, &r.Unknown); err != nil {
		return appdb.ImageScanRow{}, err
	}
	r.ScannedAt, _ = parseTS(scannedAt)
	return r, nil
}

func (s *Store) ListImageScans(ctx context.Context) ([]appdb.ImageScanRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+imageScanColumns+` FROM image_scans ORDER BY scanned_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list image scans: %w", err)
	}
	defer rows.Close()
	var out []appdb.ImageScanRow
	for rows.Next() {
		r, err := scanImageScan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) GetImageScan(ctx context.Context, imageDigest string) (appdb.ImageScanRow, error) {
	r, err := scanImageScan(s.db.QueryRowContext(ctx, `SELECT `+imageScanColumns+` FROM image_scans WHERE image_digest = ?`, imageDigest))
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.ImageScanRow{}, appdb.ErrNotFound
	}
	if err != nil {
		return appdb.ImageScanRow{}, fmt.Errorf("sqlite: get image scan: %w", err)
	}
	return r, nil
}

// ReplaceCVEResults swaps the vuln rows for a digest in a transaction.
func (s *Store) ReplaceCVEResults(ctx context.Context, imageDigest string, rows []appdb.CVEResultRow) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM cve_results WHERE image_digest = ?`, imageDigest); err != nil {
		return fmt.Errorf("sqlite: clear cve_results: %w", err)
	}
	for _, r := range rows {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO cve_results (id, image_digest, cve_id, severity, package, installed_version, fixed_version, title)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			uuid.NewString(), imageDigest, r.CVEID, r.Severity, r.Package, r.InstalledVersion, r.FixedVersion, r.Title); err != nil {
			return fmt.Errorf("sqlite: insert cve_result: %w", err)
		}
	}
	return tx.Commit()
}

func (s *Store) ListCVEResults(ctx context.Context, imageDigest string) ([]appdb.CVEResultRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, image_digest, cve_id, severity, package, installed_version, fixed_version, title
		 FROM cve_results WHERE image_digest = ?`, imageDigest)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list cve_results: %w", err)
	}
	defer rows.Close()
	var out []appdb.CVEResultRow
	for rows.Next() {
		var r appdb.CVEResultRow
		if err := rows.Scan(&r.ID, &r.ImageDigest, &r.CVEID, &r.Severity, &r.Package, &r.InstalledVersion, &r.FixedVersion, &r.Title); err != nil {
			return nil, fmt.Errorf("sqlite: scan cve_result: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
