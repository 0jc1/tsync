package main

import (
	//	"fmt
	"encoding/json"
	"fmt"
	"os"
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

func Sync(fg FileGroup) {
	
	for i, file_path := range fg.Files {
		fmt.Println(i, file_path)
	}
}

func main() {
	args := os.Args

	fmt.Println("Program", args[0])
	fmt.Println("Args:", args[1:])

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
			go Sync(group)
		}
	}
}