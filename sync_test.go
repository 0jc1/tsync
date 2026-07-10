package main

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReconcileNewestWins(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	newest := filepath.Join(dir, "newest.txt")
	missing := filepath.Join(dir, "missing.txt")
	writeFile(t, source, "old")
	writeFile(t, newest, "new")
	oldTime := time.Now().Add(-time.Minute)
	if err := os.Chtimes(source, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	engine := testEngine(SyncConfig{
		SourcePath:     source,
		Paths:          []string{newest, missing},
		ConflictPolicy: policyNewestWins,
	})
	engine.reconcile(0, "")

	assertContents(t, source, "new")
	assertContents(t, missing, "new")
}

func TestReconcileUsesChangedReplica(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	replica := filepath.Join(dir, "replica.txt")
	writeFile(t, source, "initial")
	writeFile(t, replica, "initial")

	engine := testEngine(SyncConfig{
		SourcePath:     source,
		Paths:          []string{replica},
		ConflictPolicy: policyNewestWins,
	})
	writeFile(t, replica, "edited replica")
	engine.reconcile(0, replica)
	assertContents(t, source, "edited replica")

	writeFile(t, source, "edited source")
	engine.reconcile(0, source)
	assertContents(t, replica, "edited source")
}

func TestReconcileSourceWins(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	replica := filepath.Join(dir, "replica.txt")
	writeFile(t, source, "authoritative")
	writeFile(t, replica, "replica edit")

	engine := testEngine(SyncConfig{
		SourcePath:     source,
		Paths:          []string{replica},
		ConflictPolicy: policySourceWins,
	})
	engine.reconcile(0, replica)
	assertContents(t, replica, "authoritative")
}

func TestReconcileDoesNotCreateMissingMountDirectory(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	missingParent := filepath.Join(dir, "not-mounted")
	destination := filepath.Join(missingParent, "copy.txt")
	writeFile(t, source, "content")

	engine := testEngine(SyncConfig{
		SourcePath:     source,
		Paths:          []string{destination},
		ConflictPolicy: policyNewestWins,
	})
	engine.reconcile(0, source)
	if _, err := os.Stat(missingParent); !os.IsNotExist(err) {
		t.Fatalf("missing mount directory was created or returned unexpected error: %v", err)
	}
}

func TestEngineWatchesChanges(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	replica := filepath.Join(dir, "replica.txt")
	writeFile(t, source, "initial")
	writeFile(t, replica, "initial")

	cfg := Config{
		Syncs: []SyncConfig{{
			SourcePath:     source,
			Paths:          []string{replica},
			ConflictPolicy: policyNewestWins,
		}},
		ReconcileInterval: 250 * time.Millisecond,
	}
	engine := newEngine(cfg, log.New(io.Discard, "", 0), nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- engine.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("Engine.Run() error = %v", err)
		}
	})

	time.Sleep(100 * time.Millisecond)
	writeFile(t, replica, "watched edit")
	eventually(t, 3*time.Second, func() bool {
		data, err := os.ReadFile(source)
		return err == nil && string(data) == "watched edit"
	})
}

func testEngine(entry SyncConfig) *Engine {
	cfg := Config{Syncs: []SyncConfig{entry}, ReconcileInterval: time.Second}
	return newEngine(cfg, log.New(io.Discard, "", 0), nil)
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o640); err != nil {
		t.Fatal(err)
	}
}

func assertContents(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("%s contains %q, want %q", path, data, want)
	}
}

func eventually(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}
