// Package pairing implements the daemon side of mutual trust: the
// trusted-clients store (pinned client certificate fingerprints) and the
// pairing window that gates adding new entries to it.
package pairing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const trustedClientsFileName = "trusted_clients.json"

// TrustedClient is one client certificate fingerprint the daemon has
// pinned, together with the operator-supplied label and pairing time
// recorded for audit purposes.
type TrustedClient struct {
	Fingerprint string    `json:"fingerprint"`
	Label       string    `json:"label"`
	PairedAt    time.Time `json:"paired_at"`
}

// Store is the daemon's persisted set of trusted client fingerprints.
type Store struct {
	path string

	mu      sync.RWMutex
	clients map[string]TrustedClient
}

// NewStore loads the trusted-clients store from stateDir, creating an empty
// one if it does not exist yet.
func NewStore(stateDir string) (*Store, error) {
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}

	s := &Store{
		path:    filepath.Join(stateDir, trustedClientsFileName),
		clients: map[string]TrustedClient{},
	}

	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return s, nil
	} else if err != nil {
		return nil, fmt.Errorf("read trusted clients store: %w", err)
	}

	if err := json.Unmarshal(data, &s.clients); err != nil {
		return nil, fmt.Errorf("parse trusted clients store: %w", err)
	}
	return s, nil
}

// IsTrusted reports whether fingerprint has already been pinned.
func (s *Store) IsTrusted(fingerprint string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.clients[fingerprint]
	return ok
}

// Trust pins fingerprint (with a human-readable label) and persists the
// store to disk.
func (s *Store) Trust(fingerprint, label string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.clients[fingerprint] = TrustedClient{
		Fingerprint: fingerprint,
		Label:       label,
		PairedAt:    time.Now().UTC(),
	}
	return s.persistLocked()
}

// List returns every currently trusted client.
func (s *Store) List() []TrustedClient {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]TrustedClient, 0, len(s.clients))
	for _, c := range s.clients {
		out = append(out, c)
	}
	return out
}

func (s *Store) persistLocked() error {
	data, err := json.MarshalIndent(s.clients, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal trusted clients store: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("write trusted clients store: %w", err)
	}
	return nil
}
