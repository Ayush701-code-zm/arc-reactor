package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ayushanand/tiny-aws/control-plane/internal/types"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS instances (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  image TEXT NOT NULL,
  cpu INTEGER NOT NULL,
  memory_bytes INTEGER NOT NULL,
  status TEXT NOT NULL,
  pid INTEGER NOT NULL DEFAULT 0,
  error_message TEXT NOT NULL DEFAULT ''
);`)
	return err
}

func (s *Store) InsertPending(ctx context.Context, inst types.Instance) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO instances (id, name, image, cpu, memory_bytes, status, pid, error_message)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		inst.ID, inst.Name, inst.Image, inst.CPU, inst.MemoryBytes, string(inst.Status), inst.PID, inst.ErrorMessage)
	if err != nil {
		return fmt.Errorf("insert instance: %w", err)
	}
	return nil
}

func (s *Store) UpdateLifecycle(ctx context.Context, id string, status types.Status, pid int32, errMsg string) error {
	res, err := s.db.ExecContext(ctx, `
UPDATE instances SET status = ?, pid = ?, error_message = ? WHERE id = ?`,
		string(status), pid, errMsg, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("instance %s not found", id)
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM instances WHERE id = ?`, id)
	return err
}

func (s *Store) GetByName(ctx context.Context, name string) (types.Instance, error) {
	return s.scanOne(ctx, `SELECT id, name, image, cpu, memory_bytes, status, pid, error_message FROM instances WHERE name = ?`, name)
}

func (s *Store) GetByID(ctx context.Context, id string) (types.Instance, error) {
	return s.scanOne(ctx, `SELECT id, name, image, cpu, memory_bytes, status, pid, error_message FROM instances WHERE id = ?`, id)
}

func (s *Store) List(ctx context.Context) ([]types.Instance, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, image, cpu, memory_bytes, status, pid, error_message FROM instances ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.Instance
	for rows.Next() {
		inst, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inst)
	}
	return out, rows.Err()
}

func (s *Store) scanOne(ctx context.Context, q string, arg any) (types.Instance, error) {
	row := s.db.QueryRowContext(ctx, q, arg)
	var inst types.Instance
	var status string
	err := row.Scan(&inst.ID, &inst.Name, &inst.Image, &inst.CPU, &inst.MemoryBytes, &status, &inst.PID, &inst.ErrorMessage)
	if err == sql.ErrNoRows {
		return types.Instance{}, fmt.Errorf("instance not found")
	}
	if err != nil {
		return types.Instance{}, err
	}
	inst.Status = types.Status(status)
	return inst, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRow(r rowScanner) (types.Instance, error) {
	var inst types.Instance
	var status string
	if err := r.Scan(&inst.ID, &inst.Name, &inst.Image, &inst.CPU, &inst.MemoryBytes, &status, &inst.PID, &inst.ErrorMessage); err != nil {
		return types.Instance{}, err
	}
	inst.Status = types.Status(status)
	return inst, nil
}
