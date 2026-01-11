package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// FileInfo stores metadata about a file for comparison
type FileInfo struct {
	Path    string
	Size    int64
	ModTime int64
	IsDir   bool
}

type FileInfoMap map[string]FileInfo

// scanDirectory recursively walks directory and returns file metadata
func scanDirectory(dir string) (FileInfoMap, error) {
	files := make(FileInfoMap)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("Error accessing path %s: %v\n", path, err)
			return err
		}
		// Store relative path as key
		relPath, _ := filepath.Rel(dir, path)
		files[relPath] = FileInfo{
			Path:    relPath,
			Size:    info.Size(),
			ModTime: info.ModTime().Unix(),
			IsDir:   info.IsDir(),
		}
		return nil
	})
	return files, err
}

// detectChanges compares old and new file states
func detectChanges(oldFiles, newFiles FileInfoMap) {
	// Detect added files
	for path := range newFiles {
		if _, exists := oldFiles[path]; !exists {
			fmt.Printf("[ADDED] %s\n", path)
		}
	}

	// Detect removed files
	for path := range oldFiles {
		if _, exists := newFiles[path]; !exists {
			fmt.Printf("[REMOVED] %s\n", path)
		}
	}

	// Detect modified files (size or time changed)
	for path, newInfo := range newFiles {
		if oldInfo, exists := oldFiles[path]; exists && path != "." {
			if oldInfo.Size != newInfo.Size || oldInfo.ModTime != newInfo.ModTime {
				fmt.Printf("[MODIFIED] %s\n", path)
			}
		}
	}
}

func main() {
	var args []string = os.Args
	if len(args) < 2 {
		fmt.Println("Usage: main <directory>")
		return
	}

	var dir string = args[1]
	var snapshotFile string = "data.txt"

	// Get current directory state
	start := time.Now()
	newFiles, err := scanDirectory(dir)
	elapsed := time.Since(start)
	if err != nil {
		fmt.Printf("Error scanning directory: %v\n", err)
		return
	}
	fmt.Printf("Scan completed in %.2fs\n", elapsed.Seconds())

	// Read previous snapshot if exists
	var oldFiles map[string]FileInfo
	data, err := os.ReadFile(snapshotFile)
	if err == nil {
		json.Unmarshal(data, &oldFiles)
		detectChanges(oldFiles, newFiles)
	} else {
		fmt.Println("First scan - no previous snapshot")
	}

	// Calculate total size
	var totalSize int64
	for _, file := range newFiles {
		totalSize += file.Size
	}
	totalMB := float64(totalSize) / (1024 * 1024)

	// Save current snapshot
	snapshot, _ := json.MarshalIndent(newFiles, "", "  ")
	os.WriteFile(snapshotFile, snapshot, 0644)
	fmt.Printf("Snapshot saved to data.txt\nTotal size: %.2f MB\n", totalMB)
}
