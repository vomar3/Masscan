package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"masscan/internal/models"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("db path is empty")
	}

	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return s, nil
}

func (s *Store) migrate() error {
	const schema = `
	CREATE TABLE IF NOT EXISTS current_results (
		ip TEXT NOT NULL,
		port INTEGER NOT NULL,
		service TEXT NOT NULL,
		banner TEXT NOT NULL,
		updated_at DATETIME NOT NULL,
		PRIMARY KEY (ip, port)
	);
	CREATE INDEX IF NOT EXISTS idx_current_results_updated_at ON current_results(updated_at);
	`

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("migrate sqlite schema: %w", err)
	}

	return nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) LoadCurrent(ctx context.Context) ([]models.ScanResult, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT ip, port, service, banner FROM current_results ORDER BY ip, port`)
	if err != nil {
		return nil, fmt.Errorf("query current results: %w", err)
	}
	defer rows.Close()

	var results []models.ScanResult
	for rows.Next() {
		var r models.ScanResult
		if err := rows.Scan(&r.IP, &r.Port, &r.Service, &r.Banner); err != nil {
			return nil, fmt.Errorf("scan current result: %w", err)
		}
		results = append(results, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate current results: %w", err)
	}

	return results, nil
}

func (s *Store) ReplaceCurrent(ctx context.Context, results []models.ScanResult) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM current_results`); err != nil {
		return fmt.Errorf("clear current results: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO current_results (ip, port, service, banner, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for _, r := range results {
		if _, err := stmt.ExecContext(ctx, r.IP, r.Port, r.Service, r.Banner, now); err != nil {
			return fmt.Errorf("insert current result %s:%d: %w", r.IP, r.Port, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (s *Store) ImportLegacyJSONIfEmpty(ctx context.Context, file string) error {
	if file == "" {
		return nil
	}

	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM current_results`).Scan(&count); err != nil {
		return fmt.Errorf("count current results: %w", err)
	}
	if count > 0 {
		return nil
	}

	data, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read legacy json: %w", err)
	}

	var results []models.ScanResult
	if err := json.Unmarshal(data, &results); err != nil {
		return fmt.Errorf("unmarshal legacy json: %w", err)
	}

	return s.ReplaceCurrent(ctx, results)
}
