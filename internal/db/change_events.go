package db

import (
	"fmt"
	"time"
)

// ChangeEvent is one coarse mutation notification. Kind names the
// domain that changed ("matches", "runs", "applications") — no payload,
// because consumers refetch their own state. The row's autoincrement ID
// doubles as the cursor the SSE handler tails.
type ChangeEvent struct {
	ID   int64  `json:"id" db:"id"`
	Kind string `json:"kind" db:"kind"`
	At   string `json:"at" db:"at"`
}

// AddChangeEvent publishes a mutation notification. Called by every
// writer whose change should reach an open web UI: HTTP promote/dismiss,
// and the autopilot cycle (which may run in a different process — the
// table, not an in-memory broker, is the bus). Best-effort semantics:
// a publish failure degrades live-sync, never the mutation itself.
func (s *SQLiteStore) AddChangeEvent(kind string) error {
	_, err := s.Exec(
		`INSERT INTO change_events (kind, at) VALUES (?, ?)`,
		kind, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("add change event: %w", err)
	}
	return nil
}

// ChangesSince returns events with ID greater than cursor, oldest
// first, capped at limit. An empty slice (no error) means nothing new.
func (s *SQLiteStore) ChangesSince(cursor int64, limit int) ([]ChangeEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	var events []ChangeEvent
	err := s.Select(&events,
		`SELECT id, kind, at FROM change_events WHERE id > ? ORDER BY id ASC LIMIT ?`,
		cursor, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("changes since: %w", err)
	}
	return events, nil
}

// AddChangeEvent records a change event in the fake store.
func (f *FakeStore) AddChangeEvent(kind string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.changeEventID++
	f.changeEvents = append(f.changeEvents, ChangeEvent{
		ID:   f.changeEventID,
		Kind: kind,
		At:   time.Now().UTC().Format(time.RFC3339),
	})
	return nil
}

// ChangesSince returns fake change events after the cursor.
func (f *FakeStore) ChangesSince(cursor int64, limit int) ([]ChangeEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if limit <= 0 {
		limit = 50
	}
	var out []ChangeEvent
	for _, ev := range f.changeEvents {
		if ev.ID > cursor {
			out = append(out, ev)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}
