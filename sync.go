package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Engine struct {
	config Config
	logger *log.Logger
	status *statusStore

	watcher  *fsnotify.Watcher
	watched  map[string]struct{}
	pathSets map[string]int
	hashes   map[string]cachedHash
}

type cachedHash struct {
	size    int64
	modTime time.Time
	sum     [sha256.Size]byte
}

func newEngine(config Config, logger *log.Logger, status *statusStore) *Engine {
	pathSets := make(map[string]int)
	for i, entry := range config.Syncs {
		for _, path := range entry.allPaths() {
			pathSets[filepath.Clean(path)] = i
		}
	}
	return &Engine{
		config:   config,
		logger:   logger,
		status:   status,
		watched:  make(map[string]struct{}),
		pathSets: pathSets,
		hashes:   make(map[string]cachedHash),
	}
}

func (e *Engine) Run(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create filesystem watcher: %w", err)
	}
	defer watcher.Close()
	e.watcher = watcher

	e.ensureWatches()
	for i := range e.config.Syncs {
		e.reconcile(i, "")
	}

	reconcileTicker := time.NewTicker(e.config.ReconcileInterval)
	defer reconcileTicker.Stop()
	debounceTicker := time.NewTicker(150 * time.Millisecond)
	defer debounceTicker.Stop()
	pending := make(map[int]string)

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			path := filepath.Clean(event.Name)
			set, tracked := e.pathSets[path]
			if tracked && event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename|fsnotify.Chmod) != 0 {
				delete(e.hashes, path)
				pending[set] = path
			}
		case err, ok := <-watcher.Errors:
			if ok {
				e.recordError(fmt.Errorf("filesystem watcher: %w", err))
			}
		case <-debounceTicker.C:
			for set, preferred := range pending {
				e.reconcile(set, preferred)
				delete(pending, set)
			}
		case <-reconcileTicker.C:
			e.ensureWatches()
			for i := range e.config.Syncs {
				e.reconcile(i, "")
			}
		}
	}
}

func (e *Engine) ensureWatches() {
	for path := range e.pathSets {
		dir := filepath.Dir(path)
		if _, ok := e.watched[dir]; ok {
			info, err := os.Stat(dir)
			if err == nil && info.IsDir() {
				continue
			}
			_ = e.watcher.Remove(dir)
			delete(e.watched, dir)
		}
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		if err := e.watcher.Add(dir); err != nil {
			e.recordError(fmt.Errorf("watch %q: %w", dir, err))
			continue
		}
		e.watched[dir] = struct{}{}
	}
}

func (e *Engine) reconcile(setIndex int, preferred string) {
	entry := e.config.Syncs[setIndex]
	source := entry.SourcePath

	if entry.ConflictPolicy == policyNewestWins {
		source = newestExisting(entry.allPaths(), preferred)
	}

	if source == "" {
		return
	}
	sourceInfo, err := os.Stat(source)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			e.recordError(fmt.Errorf("inspect source %q: %w", source, err))
		}
		return
	}
	if !sourceInfo.Mode().IsRegular() {
		e.recordError(fmt.Errorf("source %q is not a regular file", source))
		return
	}

	for _, destination := range entry.allPaths() {
		if destination == source {
			continue
		}
		parent, err := os.Stat(filepath.Dir(destination))
		if err != nil || !parent.IsDir() {
			continue
		}
		equal, err := e.filesEqual(source, destination)
		if err != nil {
			e.recordError(fmt.Errorf("compare %q and %q: %w", source, destination, err))
			continue
		}
		if equal {
			continue
		}
		if err := copyFileAtomic(source, destination, sourceInfo); err != nil {
			e.recordError(fmt.Errorf("sync %q to %q: %w", source, destination, err))
			continue
		}
		delete(e.hashes, destination)
		e.logger.Printf("synced %s -> %s", source, destination)
		if e.status != nil {
			e.status.recordSync(source, destination)
		}
	}
}

func newestExisting(paths []string, preferred string) string {
	var newest string
	var newestTime time.Time
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if newest == "" || info.ModTime().After(newestTime) {
			newest = path
			newestTime = info.ModTime()
		}
	}
	if preferred != "" {
		info, err := os.Stat(preferred)
		if err == nil && info.Mode().IsRegular() && !info.ModTime().Before(newestTime) {
			return preferred
		}
	}
	return newest
}

func (e *Engine) filesEqual(left, right string) (bool, error) {
	leftInfo, err := os.Stat(left)
	if err != nil {
		return false, err
	}
	rightInfo, err := os.Stat(right)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !rightInfo.Mode().IsRegular() || leftInfo.Size() != rightInfo.Size() {
		return false, nil
	}

	leftHash, err := e.cachedFileHash(left, leftInfo)
	if err != nil {
		return false, err
	}
	rightHash, err := e.cachedFileHash(right, rightInfo)
	if err != nil {
		return false, err
	}
	return leftHash == rightHash, nil
}

func (e *Engine) cachedFileHash(path string, info os.FileInfo) ([sha256.Size]byte, error) {
	if cached, ok := e.hashes[path]; ok &&
		cached.size == info.Size() && cached.modTime.Equal(info.ModTime()) {
		return cached.sum, nil
	}
	sum, err := fileHash(path)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	e.hashes[path] = cachedHash{size: info.Size(), modTime: info.ModTime(), sum: sum}
	return sum, nil
}

func fileHash(path string) ([sha256.Size]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return [sha256.Size]byte{}, err
	}
	var sum [sha256.Size]byte
	copy(sum[:], hash.Sum(nil))
	return sum, nil
}

func copyFileAtomic(source, destination string, sourceInfo os.FileInfo) (returnErr error) {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()

	parent := filepath.Dir(destination)
	temp, err := os.CreateTemp(parent, ".tsync-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer func() {
		temp.Close()
		if returnErr != nil {
			os.Remove(tempName)
		}
	}()

	if _, err := io.Copy(temp, input); err != nil {
		return err
	}
	if err := temp.Chmod(sourceInfo.Mode().Perm()); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chtimes(tempName, sourceInfo.ModTime(), sourceInfo.ModTime()); err != nil {
		return err
	}
	return os.Rename(tempName, destination)
}

func (e *Engine) recordError(err error) {
	e.logger.Printf("error: %v", err)
	if e.status != nil {
		e.status.recordError(err)
	}
}
