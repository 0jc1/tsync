package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// FileInfo stores metadata about a file for comparison
type FileInfo struct {
	Path    string
	Size    int64
	ModTime int64
	IsDir   bool
}

// scanDirectory recursively walks directory and returns file metadata
func scanDirectory(dir string) (map[string]FileInfo, error) {
	files := make(map[string]FileInfo)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
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
func detectChanges(oldFiles, newFiles map[string]FileInfo) {
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
	if len(os.Args) < 2 {
		fmt.Println("Usage: main <directory>")
		return
	}

	dir := os.Args[1]
	snapshotFile := "data.txt"

	// Get current directory state
	newFiles, err := scanDirectory(dir)
	if err != nil {
		fmt.Printf("Error scanning directory: %v\n", err)
		return
	}

	// Read previous snapshot if exists
	var oldFiles map[string]FileInfo
	data, err := os.ReadFile(snapshotFile)
	if err == nil {
		json.Unmarshal(data, &oldFiles)
		detectChanges(oldFiles, newFiles)
	} else {
		fmt.Println("First scan - no previous snapshot")
	}

	// Save current snapshot
	snapshot, _ := json.MarshalIndent(newFiles, "", "  ")
	os.WriteFile(snapshotFile, snapshot, 0644)
	fmt.Println("Snapshot saved to data.txt")
}
