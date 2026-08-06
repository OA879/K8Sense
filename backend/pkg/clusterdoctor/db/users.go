package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// isUniqueViolation reports whether err is a unique-constraint violation, across
// both SQLite ("UNIQUE constraint failed") and Postgres ("duplicate key").
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())

	return strings.Contains(msg, "unique constraint") || strings.Contains(msg, "duplicate key")
}

// User is a local-auth account. The password hash never leaves the db layer —
// GetUserAuth returns it for login verification; every other read omits it.
type User struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	Disabled  bool   `json:"disabled"`
	CreatedAt int64  `json:"createdAt"`
}

// ErrUserExists is returned when a username is already taken.
var ErrUserExists = errors.New("username already exists")

// CountUsers reports how many accounts exist — used to decide whether the
// install still needs its first-admin bootstrap.
func CountUsers(ctx context.Context, database *sql.DB) (int, error) {
	var n int
	if err := queryRow(ctx, database, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return 0, fmt.Errorf("counting users: %w", err)
	}

	return n, nil
}

// CreateUser inserts a new account with an already-hashed password.
func CreateUser(ctx context.Context, database *sql.DB, id, username, passwordHash, role string, createdAt int64) error {
	_, err := exec(ctx, database, `
		INSERT INTO users (id, username, password_hash, role, disabled, created_at)
		VALUES (?, ?, ?, ?, 0, ?)
	`, id, username, passwordHash, role, createdAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrUserExists
		}

		return fmt.Errorf("creating user: %w", err)
	}

	return nil
}

// GetUserAuth returns the account plus its password hash, for login. ok is false
// when no such username exists.
func GetUserAuth(ctx context.Context, database *sql.DB, username string) (User, string, bool, error) {
	var (
		u        User
		hash     string
		disabled int
	)

	err := queryRow(ctx, database, `
		SELECT id, username, password_hash, role, disabled, created_at
		FROM users WHERE username = ?
	`, username).Scan(&u.ID, &u.Username, &hash, &u.Role, &disabled, &u.CreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return User{}, "", false, nil
	}

	if err != nil {
		return User{}, "", false, fmt.Errorf("reading user: %w", err)
	}

	u.Disabled = disabled == 1

	return u, hash, true, nil
}

// GetUser returns an account by id (no password hash).
func GetUser(ctx context.Context, database *sql.DB, id string) (User, bool, error) {
	var (
		u        User
		disabled int
	)

	err := queryRow(ctx, database, `
		SELECT id, username, role, disabled, created_at FROM users WHERE id = ?
	`, id).Scan(&u.ID, &u.Username, &u.Role, &disabled, &u.CreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return User{}, false, nil
	}

	if err != nil {
		return User{}, false, fmt.Errorf("reading user: %w", err)
	}

	u.Disabled = disabled == 1

	return u, true, nil
}

// ListUsers returns every account, newest first (no password hashes).
func ListUsers(ctx context.Context, database *sql.DB) ([]User, error) {
	rows, err := query(ctx, database, `
		SELECT id, username, role, disabled, created_at FROM users ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	defer rows.Close()

	var users []User

	for rows.Next() {
		var (
			u        User
			disabled int
		)

		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &disabled, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning user row: %w", err)
		}

		u.Disabled = disabled == 1
		users = append(users, u)
	}

	return users, rows.Err()
}

// SetUserRole updates an account's role.
func SetUserRole(ctx context.Context, database *sql.DB, id, role string) error {
	_, err := exec(ctx, database, `UPDATE users SET role = ? WHERE id = ?`, role, id)
	if err != nil {
		return fmt.Errorf("updating user role: %w", err)
	}

	return nil
}

// SetUserDisabled enables/disables an account.
func SetUserDisabled(ctx context.Context, database *sql.DB, id string, disabled bool) error {
	d := 0
	if disabled {
		d = 1
	}

	_, err := exec(ctx, database, `UPDATE users SET disabled = ? WHERE id = ?`, d, id)
	if err != nil {
		return fmt.Errorf("updating user status: %w", err)
	}

	return nil
}

// SetUserPassword replaces an account's password hash.
func SetUserPassword(ctx context.Context, database *sql.DB, id, passwordHash string) error {
	_, err := exec(ctx, database, `UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, id)
	if err != nil {
		return fmt.Errorf("updating password: %w", err)
	}

	return nil
}

// GetUserAuthByName is a test/convenience wrapper returning just the user for a
// username (no hash), used where only the identity is needed.
func GetUserAuthByName(ctx context.Context, database *sql.DB, username string) (User, error) {
	u, _, _, err := GetUserAuth(ctx, database, username)
	return u, err
}
