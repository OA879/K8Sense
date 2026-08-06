package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// CreateSession stores an opaque session token for a user with an expiry.
func CreateSession(ctx context.Context, database *sql.DB, token, userID string, createdAt, expiresAt int64) error {
	_, err := exec(ctx, database, `
		INSERT INTO sessions (token, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)
	`, token, userID, createdAt, expiresAt)
	if err != nil {
		return fmt.Errorf("creating session: %w", err)
	}

	return nil
}

// SessionUser resolves a session token to its user id, if the session exists and
// has not expired (now is the current unix time). ok is false otherwise.
func SessionUser(ctx context.Context, database *sql.DB, token string, now int64) (string, bool, error) {
	var (
		userID  string
		expires int64
	)

	err := queryRow(ctx, database, `
		SELECT user_id, expires_at FROM sessions WHERE token = ?
	`, token).Scan(&userID, &expires)

	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}

	if err != nil {
		return "", false, fmt.Errorf("reading session: %w", err)
	}

	if now >= expires {
		return "", false, nil
	}

	return userID, true, nil
}

// DeleteSession removes a single session (logout).
func DeleteSession(ctx context.Context, database *sql.DB, token string) error {
	_, err := exec(ctx, database, `DELETE FROM sessions WHERE token = ?`, token)
	if err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}

	return nil
}

// DeleteUserSessions revokes every session for a user (on disable / password
// change / forced logout).
func DeleteUserSessions(ctx context.Context, database *sql.DB, userID string) error {
	_, err := exec(ctx, database, `DELETE FROM sessions WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("revoking user sessions: %w", err)
	}

	return nil
}

// PurgeExpiredSessions deletes sessions past their expiry.
func PurgeExpiredSessions(ctx context.Context, database *sql.DB, now int64) error {
	_, err := exec(ctx, database, `DELETE FROM sessions WHERE expires_at <= ?`, now)
	if err != nil {
		return fmt.Errorf("purging expired sessions: %w", err)
	}

	return nil
}
