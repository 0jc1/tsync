package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadSingleEntryConfig(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	destination := filepath.Join(dir, "copy.txt")
	path := writeConfig(t, dir, `{
		"source_path": `+quote(source)+`,
		"paths": [`+quote(destination)+`]
	}`)

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if len(cfg.Syncs) != 1 {
		t.Fatalf("got %d entries, want 1", len(cfg.Syncs))
	}
	if cfg.Syncs[0].ConflictPolicy != policyNewestWins {
		t.Fatalf("default policy = %q", cfg.Syncs[0].ConflictPolicy)
	}
	if cfg.ReconcileInterval != 2*time.Second {
		t.Fatalf("default interval = %v", cfg.ReconcileInterval)
	}
}

func TestLoadMultipleEntryConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, `{
		"reconcile_interval": "500ms",
		"syncs": [
			{
				"source_path": `+quote(filepath.Join(dir, "one"))+`,
				"paths": [`+quote(filepath.Join(dir, "one-copy"))+`],
				"conflict_policy": "source_wins"
			},
			{
				"source_path": `+quote(filepath.Join(dir, "two"))+`,
				"paths": [`+quote(filepath.Join(dir, "two-copy"))+`]
			}
		]
	}`)

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if len(cfg.Syncs) != 2 || cfg.ReconcileInterval != 500*time.Millisecond {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestConfigValidationErrors(t *testing.T) {
	dir := t.TempDir()
	tests := map[string]string{
		"relative source": `{"source_path":"relative","paths":["/tmp/copy"]}`,
		"no paths":        `{"source_path":"/tmp/source","paths":[]}`,
		"bad policy":      `{"source_path":"/tmp/source","paths":["/tmp/copy"],"conflict_policy":"merge"}`,
		"unknown field":   `{"source_path":"/tmp/source","paths":["/tmp/copy"],"extra":true}`,
		"duplicate":       `{"source_path":"/tmp/source","paths":["/tmp/source"]}`,
		"two values":      `{"source_path":"/tmp/source","paths":["/tmp/copy"]} {}`,
		"directory path":  `{"source_path":` + quote(dir) + `,"paths":["/tmp/copy"]}`,
	}
	for name, contents := range tests {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, dir, contents)
			if _, err := loadConfig(path); err == nil {
				t.Fatal("loadConfig() unexpectedly succeeded")
			}
		})
	}
}

func writeConfig(t *testing.T, dir, contents string) string {
	t.Helper()
	name := strings.ReplaceAll(t.Name(), "/", "-") + ".json"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func quote(value string) string {
	return `"` + strings.ReplaceAll(value, `\`, `\\`) + `"`
}
