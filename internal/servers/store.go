// Package servers provides a thread-safe store for the latest DZSA sync result per port.
package servers

import (
	"sort"
	"sync"

	"github.com/jsirianni/dzsa-sync/model"
)

// Store holds the latest DZSA query result per config port. Safe for concurrent use.
type Store struct {
	mu     sync.RWMutex
	byPort map[int]*model.Result
	ports  map[int]bool
}

// New returns a store that only accepts and returns data for the given config ports.
func New(ports []int) *Store {
	valid := make(map[int]bool, len(ports))
	for _, p := range ports {
		valid[p] = true
	}
	return &Store{
		byPort: make(map[int]*model.Result),
		ports:  valid,
	}
}

// Set stores the result for the given port. Port must be in the set passed to New; otherwise Set is a no-op.
func (s *Store) Set(port int, result *model.Result) {
	if result == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ports[port] {
		// Copy so callers cannot mutate after Set
		cp := *result
		s.byPort[port] = &cp
	}
}

// Get returns the stored result for the port and true if found. Returns (nil, false) if port is not a valid config port or no data yet.
func (s *Store) Get(port int) (*model.Result, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.ports[port] {
		return nil, false
	}
	r, ok := s.byPort[port]
	if !ok || r == nil {
		return nil, false
	}
	cp := *r
	return &cp, true
}

// ServerEntry is a single server in the list response (port + result).
type ServerEntry struct {
	Port   int           `json:"port"`
	Result *model.Result `json:"result"`
}

// GetAll returns all stored results as a slice of ServerEntry, one per valid port that has data, in stable order (by port).
func (s *Store) GetAll() []ServerEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Iterate in deterministic order: use sorted ports. We don't have ports as slice here, so collect from byPort keys and the valid set.
	var entries []ServerEntry
	for port, r := range s.byPort {
		if !s.ports[port] || r == nil {
			continue
		}
		cp := *r
		entries = append(entries, ServerEntry{Port: port, Result: &cp})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Port < entries[j].Port })
	return entries
}
