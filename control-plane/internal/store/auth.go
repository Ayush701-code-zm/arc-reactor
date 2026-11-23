package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ayushanand/tiny-aws/control-plane/internal/types"
)

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM users`).Scan(&n)
	return n, err
}

func (s *Store) BootstrapUser(ctx context.Context, user types.User) (types.User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return types.User{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM users`).Scan(&n); err != nil {
		return types.User{}, err
	}
	if n > 0 {
		return types.User{}, fmt.Errorf("conflict: bootstrap already completed")
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO users (id, username, password_hash, role) VALUES (?, ?, ?, ?)`,
		user.ID, user.Username, user.PasswordHash, user.Role); err != nil {
		return types.User{}, err
	}
	if err := tx.Commit(); err != nil {
		return types.User{}, err
	}
	return user, nil
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (types.User, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, username, password_hash, role FROM users WHERE username = ?`, username)
	return scanUser(row)
}

func (s *Store) GetUserByID(ctx context.Context, id string) (types.User, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, username, password_hash, role FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func scanUser(r rowScanner) (types.User, error) {
	var u types.User
	err := r.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role)
	if err == sql.ErrNoRows {
		return types.User{}, fmt.Errorf("user not found")
	}
	if err != nil {
		return types.User{}, err
	}
	return u, nil
}

func (s *Store) InsertAccessSession(ctx context.Context, sess types.AccessSession) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO access_sessions (token_hash, user_id, expires_at) VALUES (?, ?, ?)`,
		sess.TokenHash, sess.UserID, sess.ExpiresAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) GetAccessSession(ctx context.Context, tokenHash string) (types.AccessSession, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT token_hash, user_id, expires_at FROM access_sessions WHERE token_hash = ?`, tokenHash)
	var sess types.AccessSession
	var exp string
	err := row.Scan(&sess.TokenHash, &sess.UserID, &exp)
	if err == sql.ErrNoRows {
		return types.AccessSession{}, fmt.Errorf("session not found")
	}
	if err != nil {
		return types.AccessSession{}, err
	}
	t, err := time.Parse(time.RFC3339Nano, exp)
	if err != nil {
		t, err = time.Parse(time.RFC3339, exp)
		if err != nil {
			return types.AccessSession{}, err
		}
	}
	sess.ExpiresAt = t
	return sess, nil
}

func (s *Store) DeleteAccessSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM access_sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (s *Store) DeleteAccessByUser(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM access_sessions WHERE user_id = ?`, userID)
	return err
}

func (s *Store) InsertRefreshSession(ctx context.Context, sess types.RefreshSession) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO refresh_sessions (token_hash, user_id, expires_at) VALUES (?, ?, ?)`,
		sess.TokenHash, sess.UserID, sess.ExpiresAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) GetRefreshSession(ctx context.Context, tokenHash string) (types.RefreshSession, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT token_hash, user_id, expires_at FROM refresh_sessions WHERE token_hash = ?`, tokenHash)
	var sess types.RefreshSession
	var exp string
	err := row.Scan(&sess.TokenHash, &sess.UserID, &exp)
	if err == sql.ErrNoRows {
		return types.RefreshSession{}, fmt.Errorf("session not found")
	}
	if err != nil {
		return types.RefreshSession{}, err
	}
	t, err := time.Parse(time.RFC3339Nano, exp)
	if err != nil {
		t, err = time.Parse(time.RFC3339, exp)
		if err != nil {
			return types.RefreshSession{}, err
		}
	}
	sess.ExpiresAt = t
	return sess, nil
}

func (s *Store) DeleteRefreshSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM refresh_sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (s *Store) AdoptUnownedResources(ctx context.Context, ownerID string) error {
	stmts := []string{
		`UPDATE instances SET owner_id = ? WHERE owner_id = '' OR owner_id IS NULL`,
		`UPDATE images SET owner_id = ? WHERE owner_id = '' OR owner_id IS NULL`,
		`UPDATE volumes SET owner_id = ? WHERE owner_id = '' OR owner_id IS NULL`,
		`UPDATE networks SET owner_id = ? WHERE owner_id = '' OR owner_id IS NULL`,
	}
	for _, q := range stmts {
		if _, err := s.db.ExecContext(ctx, q, ownerID); err != nil {
			return err
		}
	}
	return nil
}
