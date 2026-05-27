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

func (s *Store) UpsertUserTOTP(ctx context.Context, t appdb.UserTOTP) error {
	codes, _ := json.Marshal(t.RecoveryHashes)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO user_totp (user_id, secret_encrypted, enabled, recovery_codes, created_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET secret_encrypted=excluded.secret_encrypted,
		   enabled=excluded.enabled, recovery_codes=excluded.recovery_codes`,
		t.UserID, t.SecretEncrypted, boolToInt(t.Enabled), string(codes), tsText(time.Now()))
	if err != nil {
		return fmt.Errorf("sqlite: upsert user_totp: %w", err)
	}
	return nil
}

func (s *Store) GetUserTOTP(ctx context.Context, userID string) (appdb.UserTOTP, error) {
	var t appdb.UserTOTP
	var enabled int
	var codes string
	err := s.db.QueryRowContext(ctx,
		`SELECT user_id, secret_encrypted, enabled, recovery_codes FROM user_totp WHERE user_id = ?`, userID).
		Scan(&t.UserID, &t.SecretEncrypted, &enabled, &codes)
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.UserTOTP{}, appdb.ErrNotFound
	}
	if err != nil {
		return appdb.UserTOTP{}, fmt.Errorf("sqlite: get user_totp: %w", err)
	}
	t.Enabled = enabled != 0
	_ = json.Unmarshal([]byte(codes), &t.RecoveryHashes)
	return t, nil
}

func (s *Store) DeleteUserTOTP(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM user_totp WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("sqlite: delete user_totp: %w", err)
	}
	return nil
}
