// Package store persists the client's list of known daemons: address,
// pinned certificate fingerprint, a human label, and pairing time.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const knownServersFileName = "known_servers.json"

// Server is one daemon the client has paired with.
type Server struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Address     string    `json:"address"`
	Fingerprint string    `json:"fingerprint"`
	PairedAt    time.Time `json:"paired_at"`
}

// IDFromFingerprint derives a stable, URL-safe server ID from its pinned
// certificate fingerprint, so the same daemon always maps to the same
// /api/servers/{id} path.
func IDFromFingerprint(fingerprint string) string {
	return strings.ReplaceAll(fingerprint, ":", "")
}

// Store is the client's persisted set of known/paired servers.
type Store struct {
	path string

	mu      sync.RWMutex
	servers map[string]Server
}

// NewStore loads the known-servers store from dir, creating an empty one if
// it does not exist yet.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}

	s := &Store{
		path:    filepath.Join(dir, knownServersFileName),
		servers: map[string]Server{},
	}

	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return s, nil
	} else if err != nil {
		return nil, fmt.Errorf("read known servers store: %w", err)
	}

	if err := json.Unmarshal(data, &s.servers); err != nil {
		return nil, fmt.Errorf("parse known servers store: %w", err)
	}
	return s, nil
}

// List returns every known server.
func (s *Store) List() []Server {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Server, 0, len(s.servers))
	for _, srv := range s.servers {
		out = append(out, srv)
	}
	return out
}

// Get returns the server with the given ID, if known.
func (s *Store) Get(id string) (Server, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	srv, ok := s.servers[id]
	return srv, ok
}

// Add pins/updates srv and persists the store to disk.
func (s *Store) Add(srv Server) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.servers[srv.ID] = srv
	return s.persistLocked()
}

// Remove forgets the server with the given ID.
func (s *Store) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.servers, id)
	return s.persistLocked()
}

func (s *Store) persistLocked() error {
	data, err := json.MarshalIndent(s.servers, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal known servers store: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("write known servers store: %w", err)
	}
	return nil
}
