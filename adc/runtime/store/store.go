package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate() error {
	ddl := `
CREATE TABLE IF NOT EXISTS runs (
  run_id TEXT PRIMARY KEY,
  scenario_name TEXT NOT NULL,
  started_at TEXT NOT NULL,
  finished_at TEXT,
  status TEXT NOT NULL,
  final_state_json TEXT,
  evidence_json TEXT
);
CREATE TABLE IF NOT EXISTS events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id TEXT NOT NULL,
  turn_index INTEGER NOT NULL,
  step_index INTEGER NOT NULL,
  role TEXT NOT NULL,
  action_type TEXT NOT NULL,
  payload_json TEXT NOT NULL,
  response_json TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY (run_id) REFERENCES runs(run_id)
);
`
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("migrate sqlite: %w", err)
	}
	return nil
}

func (s *Store) CreateRun(runID, scenarioName string) error {
	_, err := s.db.Exec(
		`INSERT INTO runs(run_id, scenario_name, started_at, status) VALUES(?,?,?,?)`,
		runID,
		scenarioName,
		time.Now().UTC().Format(time.RFC3339),
		"running",
	)
	if err != nil {
		return fmt.Errorf("insert run: %w", err)
	}
	return nil
}

func (s *Store) AppendEvent(
	runID string,
	turnIndex int,
	stepIndex int,
	role string,
	actionType string,
	payload map[string]any,
	response map[string]any,
) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT INTO events(run_id, turn_index, step_index, role, action_type, payload_json, response_json, created_at) VALUES(?,?,?,?,?,?,?,?)`,
		runID,
		turnIndex,
		stepIndex,
		role,
		actionType,
		string(payloadJSON),
		string(responseJSON),
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

func (s *Store) FinishRun(runID string, status string, finalState map[string]any, evidence map[string]any) error {
	finalStateJSON, err := json.Marshal(finalState)
	if err != nil {
		return fmt.Errorf("marshal final state: %w", err)
	}
	evidenceJSON, err := json.Marshal(evidence)
	if err != nil {
		return fmt.Errorf("marshal evidence: %w", err)
	}
	_, err = s.db.Exec(
		`UPDATE runs SET finished_at=?, status=?, final_state_json=?, evidence_json=? WHERE run_id=?`,
		time.Now().UTC().Format(time.RFC3339),
		status,
		string(finalStateJSON),
		string(evidenceJSON),
		runID,
	)
	if err != nil {
		return fmt.Errorf("finish run: %w", err)
	}
	return nil
}
