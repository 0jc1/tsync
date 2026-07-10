package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

type runtimeStatus struct {
	PID             int       `json:"pid"`
	Running         bool      `json:"running"`
	StartedAt       time.Time `json:"started_at"`
	StoppedAt       time.Time `json:"stopped_at,omitempty"`
	LastSyncAt      time.Time `json:"last_sync_at,omitempty"`
	LastSource      string    `json:"last_source,omitempty"`
	LastDestination string    `json:"last_destination,omitempty"`
	LastError       string    `json:"last_error,omitempty"`
	LastErrorAt     time.Time `json:"last_error_at,omitempty"`
}

type statusStore struct {
	path  string
	mu    sync.Mutex
	state runtimeStatus
}

func newStatusStore(configPath string) *statusStore {
	return &statusStore{
		path: filepath.Join(filepath.Dir(configPath), "status.json"),
		state: runtimeStatus{
			PID:       os.Getpid(),
			Running:   true,
			StartedAt: time.Now(),
		},
	}
}

func (s *statusStore) start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeLocked()
}

func (s *statusStore) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Running = false
	s.state.StoppedAt = time.Now()
	_ = s.writeLocked()
}

func (s *statusStore) recordSync(source, destination string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.LastSyncAt = time.Now()
	s.state.LastSource = source
	s.state.LastDestination = destination
	s.state.LastError = ""
	s.state.LastErrorAt = time.Time{}
	_ = s.writeLocked()
}

func (s *statusStore) recordError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.LastError = err.Error()
	s.state.LastErrorAt = time.Now()
	_ = s.writeLocked()
}

func (s *statusStore) writeLocked() error {
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(s.path), ".tsync-status-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err := temp.Write(append(data, '\n')); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, s.path)
}

func readRuntimeStatus(configPath string) (runtimeStatus, error) {
	path := filepath.Join(filepath.Dir(configPath), "status.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return runtimeStatus{}, nil
	}
	if err != nil {
		return runtimeStatus{}, fmt.Errorf("read status: %w", err)
	}
	var state runtimeStatus
	if err := json.Unmarshal(data, &state); err != nil {
		return runtimeStatus{}, fmt.Errorf("parse status: %w", err)
	}
	if state.Running && !processRunning(state.PID) {
		state.Running = false
	}
	return state, nil
}

func processRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
