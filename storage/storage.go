package storage

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// DataStore defines the interface for persistence
type DataStore interface {
	SaveRequest(profileURL string) error
	IsRequestSent(profileURL string) bool

	SaveMessage(profileURL string) error
	IsMessaged(profileURL string) bool

	SaveConnection(profileURL string) error
	IsConnected(profileURL string) bool

	Close() error
}

// MemoryStore implements DataStore with JSON file backing
type MemoryStore struct {
	mu   sync.RWMutex
	File string
	Data StateData
}

type StateData struct {
	Requests    map[string]time.Time `json:"requests"`
	Messages    map[string]time.Time `json:"messages"`
	Connections map[string]time.Time `json:"connections"`
}

// NewJSONStore creates a new store backed by a JSON file
func NewJSONStore(filepath string) (*MemoryStore, error) {
	s := &MemoryStore{
		File: filepath,
		Data: StateData{
			Requests:    make(map[string]time.Time),
			Messages:    make(map[string]time.Time),
			Connections: make(map[string]time.Time),
		},
	}

	// Load if exists
	if _, err := os.Stat(filepath); err == nil {
		content, err := os.ReadFile(filepath)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(content, &s.Data); err != nil {
			return nil, err
		}
	}

	return s, nil
}

func (s *MemoryStore) persist() error {
	data, err := json.MarshalIndent(s.Data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.File, data, 0644)
}

// SaveRequest records a sent connection request
func (s *MemoryStore) SaveRequest(profileURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Data.Requests[profileURL] = time.Now()
	return s.persist()
}

func (s *MemoryStore) IsRequestSent(profileURL string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.Data.Requests[profileURL]
	return exists
}

// SaveMessage records a sent message
func (s *MemoryStore) SaveMessage(profileURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Data.Messages[profileURL] = time.Now()
	return s.persist()
}

func (s *MemoryStore) IsMessaged(profileURL string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.Data.Messages[profileURL]
	return exists
}

// SaveConnection records a confirmed connection
func (s *MemoryStore) SaveConnection(profileURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Data.Connections[profileURL] = time.Now()
	return s.persist()
}

func (s *MemoryStore) IsConnected(profileURL string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.Data.Connections[profileURL]
	return exists
}

func (s *MemoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.persist()
}
