package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	policyNewestWins = "newest_wins"
	policySourceWins = "source_wins"
)

type Config struct {
	Syncs             []SyncConfig
	ReconcileInterval time.Duration
}

type SyncConfig struct {
	SourcePath     string   `json:"source_path"`
	Paths          []string `json:"paths"`
	ConflictPolicy string   `json:"conflict_policy,omitempty"`
}

type configFile struct {
	Syncs             []SyncConfig `json:"syncs"`
	ReconcileInterval string       `json:"reconcile_interval,omitempty"`

	// These fields allow a single sync entry at the top level.
	SourcePath     string   `json:"source_path,omitempty"`
	Paths          []string `json:"paths,omitempty"`
	ConflictPolicy string   `json:"conflict_policy,omitempty"`
}

func defaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return filepath.Join(home, ".config", "tsync", "config.json"), nil
}

func loadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	var raw configFile
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return Config{}, fmt.Errorf("parse config %q: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return Config{}, fmt.Errorf("parse config %q: multiple JSON values", path)
		}
		return Config{}, fmt.Errorf("parse config %q: %w", path, err)
	}

	cfg := Config{Syncs: raw.Syncs, ReconcileInterval: 2 * time.Second}
	if raw.SourcePath != "" || len(raw.Paths) > 0 || raw.ConflictPolicy != "" {
		if len(raw.Syncs) > 0 {
			return Config{}, errors.New("config cannot contain both top-level source_path and syncs")
		}
		cfg.Syncs = []SyncConfig{{
			SourcePath:     raw.SourcePath,
			Paths:          raw.Paths,
			ConflictPolicy: raw.ConflictPolicy,
		}}
	}
	if raw.ReconcileInterval != "" {
		interval, err := time.ParseDuration(raw.ReconcileInterval)
		if err != nil {
			return Config{}, fmt.Errorf("invalid reconcile_interval: %w", err)
		}
		cfg.ReconcileInterval = interval
	}

	if err := validateConfig(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func validateConfig(cfg *Config) error {
	if len(cfg.Syncs) == 0 {
		return errors.New("config must contain at least one sync entry")
	}
	if cfg.ReconcileInterval < 250*time.Millisecond {
		return errors.New("reconcile_interval must be at least 250ms")
	}

	used := make(map[string]int)
	for i := range cfg.Syncs {
		entry := &cfg.Syncs[i]
		entry.SourcePath = filepath.Clean(entry.SourcePath)
		if entry.SourcePath == "." || !filepath.IsAbs(entry.SourcePath) {
			return fmt.Errorf("syncs[%d].source_path must be an absolute path", i)
		}
		if len(entry.Paths) == 0 {
			return fmt.Errorf("syncs[%d].paths must contain at least one destination", i)
		}
		if entry.ConflictPolicy == "" {
			entry.ConflictPolicy = policyNewestWins
		}
		entry.ConflictPolicy = strings.ToLower(entry.ConflictPolicy)
		if entry.ConflictPolicy != policyNewestWins && entry.ConflictPolicy != policySourceWins {
			return fmt.Errorf("syncs[%d].conflict_policy must be %q or %q", i, policyNewestWins, policySourceWins)
		}

		allPaths := append([]string{entry.SourcePath}, entry.Paths...)
		seen := make(map[string]struct{}, len(allPaths))
		for j, path := range allPaths {
			path = filepath.Clean(path)
			if !filepath.IsAbs(path) {
				return fmt.Errorf("syncs[%d] path %q must be absolute", i, path)
			}
			if _, ok := seen[path]; ok {
				return fmt.Errorf("syncs[%d] contains duplicate path %q", i, path)
			}
			if previous, ok := used[path]; ok {
				return fmt.Errorf("path %q is also used by syncs[%d]", path, previous)
			}
			if info, err := os.Stat(path); err == nil && !info.Mode().IsRegular() {
				return fmt.Errorf("syncs[%d] path %q is not a regular file", i, path)
			}
			seen[path] = struct{}{}
			used[path] = i
			if j > 0 {
				entry.Paths[j-1] = path
			}
		}
	}
	return nil
}

func (s SyncConfig) allPaths() []string {
	paths := make([]string, 0, len(s.Paths)+1)
	paths = append(paths, s.SourcePath)
	return append(paths, s.Paths...)
}
