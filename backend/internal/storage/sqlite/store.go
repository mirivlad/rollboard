package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"rollboard/internal/game"
)

type Store struct {
	db *sql.DB
}

func dbPathFromDSN(dsn string) string {
	if idx := strings.Index(dsn, "?"); idx >= 0 {
		return dsn[:idx]
	}
	return dsn
}

func New(dsn string) (*Store, error) {
	dbPath := dbPathFromDSN(dsn)
	log.Printf("opening database: %s", dbPath)

	// Ensure the directory for the database file exists
	if dir := filepath.Dir(dbPath); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create db directory %s: %w", dir, err)
		}
	}

	db, err := sql.Open("sqlite3", dsn+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		if recovered := tryRecoverWAL(dsn); recovered {
			log.Printf("WAL recovery performed, retrying open")
			db, err = sql.Open("sqlite3", dsn+"?_journal_mode=WAL&_foreign_keys=on")
			if err != nil {
				return nil, fmt.Errorf("open db after WAL recovery: %w", err)
			}
			if err := db.Ping(); err != nil {
				db.Close()
				return nil, fmt.Errorf("ping db after WAL recovery: %w", err)
			}
		} else {
			return nil, fmt.Errorf("ping db: %w", err)
		}
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// tryRecoverWAL attempts to recover from stale WAL/SHM files.
// It renames .db-wal and .db-shm to .bak.<timestamp> if they exist.
// Returns true if recovery was attempted (files may or may not have existed).
func tryRecoverWAL(dsn string) bool {
	dbPath := dbPathFromDSN(dsn)
	if dbPath == "" {
		return false
	}
	walPath := dbPath + "-wal"
	shmPath := dbPath + "-shm"

	walExists := fileExists(walPath)
	shmExists := fileExists(shmPath)

	if !walExists && !shmExists {
		return false
	}

	ts := time.Now().UnixMilli()
	recovered := false

	if walExists {
		bak := fmt.Sprintf("%s.bak.%d", walPath, ts)
		if err := os.Rename(walPath, bak); err != nil {
			log.Printf("warning: failed to rename stale WAL file %s: %v", walPath, err)
		} else {
			log.Printf("renamed stale WAL file %s -> %s", walPath, bak)
			recovered = true
		}
	}
	if shmExists {
		bak := fmt.Sprintf("%s.bak.%d", shmPath, ts)
		if err := os.Rename(shmPath, bak); err != nil {
			log.Printf("warning: failed to rename stale SHM file %s: %v", shmPath, err)
		} else {
			log.Printf("renamed stale SHM file %s -> %s", shmPath, bak)
			recovered = true
		}
	}

	return recovered
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS games (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			version INTEGER NOT NULL DEFAULT 1,
			definition_json TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			game_id TEXT NOT NULL,
			game_version INTEGER NOT NULL,
			session_json TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	return err
}

type GameSummary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Version   int    `json:"version"`
	UpdatedAt string `json:"updatedAt"`
}

func (s *Store) ListGames() ([]GameSummary, error) {
	rows, err := s.db.Query(`SELECT id, title, version, updated_at FROM games ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []GameSummary
	for rows.Next() {
		var g GameSummary
		var updatedAt time.Time
		if err := rows.Scan(&g.ID, &g.Title, &g.Version, &updatedAt); err != nil {
			return nil, err
		}
		g.UpdatedAt = updatedAt.Format(time.RFC3339)
		list = append(list, g)
	}
	return list, nil
}

func (s *Store) GetGame(id string) (*game.GameDefinition, error) {
	var defJSON string
	err := s.db.QueryRow(`SELECT definition_json FROM games WHERE id = ?`, id).Scan(&defJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	var g game.GameDefinition
	if err := json.Unmarshal([]byte(defJSON), &g); err != nil {
		return nil, err
	}
	return &g, nil
}

func (s *Store) CreateGame(g *game.GameDefinition) error {
	defJSON, err := json.Marshal(g)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT INTO games (id, title, version, definition_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		g.ID, g.Title, g.Version, string(defJSON), time.Now().UTC(), time.Now().UTC(),
	)
	return err
}

func (s *Store) UpdateGame(g *game.GameDefinition) error {
	g.Version++
	defJSON, err := json.Marshal(g)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`UPDATE games SET title = ?, version = ?, definition_json = ?, updated_at = ? WHERE id = ?`,
		g.Title, g.Version, string(defJSON), time.Now().UTC(), g.ID,
	)
	return err
}

func (s *Store) SaveSession(session *game.GameSession) error {
	sessionJSON, err := json.Marshal(session)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT OR REPLACE INTO sessions (id, game_id, game_version, session_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		session.ID, session.GameID, session.GameVersion, string(sessionJSON), time.Now().UTC(), time.Now().UTC(),
	)
	return err
}

func (s *Store) GetSession(id string) (*game.GameSession, error) {
	var sessionJSON string
	var gameVersion int
	err := s.db.QueryRow(`SELECT session_json, game_version FROM sessions WHERE id = ?`, id).Scan(&sessionJSON, &gameVersion)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	var session game.GameSession
	if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
		return nil, err
	}
	return &session, nil
}
