package main

import (
	//	"fmt
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)


type FileGroup struct {
	Source string  `json:"source_file_path"`
	Policy string  `json:"policy"`  // newest_wins, source_wins
	Files []string `json:"file_paths"`
}

type Config struct {
	RefreshInterval int  `json:"refresh_interval"`
	FileGroups []FileGroup `json:"file_groups"`
}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func trace() {
    _, file, line, _ := runtime.Caller(1)
    log.Printf("reached %s:%d", filepath.Base(file), line)
}

func Sync(fg FileGroup, wg *sync.WaitGroup) {
	defer wg.Done()

	var files []*os.File
	policy := fg.Policy
	source, err := os.OpenFile(fg.Source, os.O_RDWR, 0644)

	var newestFile *os.File
	var newestModTime time.Time
	
	if err != nil && policy == "source_wins" {
		fmt.Println("Cannot sync file group without source", err)
		trace()
		return
	} 

	for _, file_path := range fg.Files {
		file, err := os.OpenFile(file_path, os.O_RDWR, 0644)
		if err != nil {
			log.Println("err", err)
			trace()
		}

		if info, err := file.Stat(); err == nil {
			if info.ModTime().After(newestModTime) {
				newestModTime = info.ModTime()
				newestFile = file 
			}
		}
		
		files = append(files, file)
	}

	if policy == "source_wins" {
		source_info, err := source.Stat()
		if err != nil {
			fmt.Println(err)
			trace()
			return
		}

		for _, file := range files {
			info, err := file.Stat()
			defer file.Close()
			if err != nil {
				fmt.Println(err)
				trace()
				continue
			}

			if info.Size() != source_info.Size() {
				source.Seek(0, io.SeekStart)
				//clear file
				file.Truncate(0)
				_, err := io.Copy(file, source)
				if err != nil {
					fmt.Println(err)
					trace()
				} else {
					fmt.Println("copied")
				}
			}
		}
		source.Close()
	} else if policy == "newest_wins" {
		newest_info, err := newestFile.Stat()
		if err != nil {
			fmt.Println(err)
			trace()
			return
		}

		for _, file := range files {
			info, err := file.Stat()
			defer file.Close()
			if err != nil {
				fmt.Println(err)
				trace()
				continue
			}

			if info.ModTime().Before(newestModTime) && info.Size() != newest_info.Size() {
				fmt.Printf("%s modtime before newest\n", file.Name())
				file.Truncate(0)

				_, err := io.Copy(file, newestFile)
				if err != nil {
					fmt.Println(err)
					trace()
				} else {
					fmt.Println("copied")
				}
			} 			
		}
	}
}

func main() {
	args := os.Args
	fmt.Println("Program", args[0])
	fmt.Println("Args:", args[1:])

	var wg sync.WaitGroup
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		// fires every interval

		config, err := Load("config.json")
		if err != nil {
			fmt.Println("Failed to load config", err)
			continue
		}
		
		// update interval
		ticker.Reset(time.Duration(config.RefreshInterval) * time.Second)
		
		// sync all file groups
		for _, group := range config.FileGroups {
			//check if file group is valid 
			if len(group.Files) == 0 {
				fmt.Println("Invalid group: No file_paths in group")
				continue
			}
			wg.Add(1)
			go Sync(group, &wg)
		}
		wg.Wait()
	}
}