package main

import (
	"fmt"
	"os"
	"time"
	"io"
)


func main() {
    pid := os.Getpid()
    fmt.Println("Process ID:", pid)

	var loopDelay time.Duration = 1000*time.Millisecond

	var args []string = os.Args
	if len(args) < 3 {
		fmt.Println("Usage: main <source> <dest>")
		return
	}

	var source string = args[1]
	var dest string = args[2]

    fmt.Printf("Source: %s\n", source)
    fmt.Printf("Destination: %s\n", dest)

	
	info, err := os.Stat(source)
	if os.IsNotExist(err) {
		fmt.Println("source does not exist.", err.Error())
		os.Exit(1)
	}

	if err == nil {
		fmt.Println("source file")
		fmt.Println("bytes: ", info.Size())    // bytes
		fmt.Println("mod time: " + info.ModTime().Format(time.DateTime)) // time.Time
		fmt.Println("is dir: ", info.IsDir())   // bool
		fmt.Println("mode: ", info.Mode())    // os.FileMode
		fmt.Println()
	}
	
	destInfo, err1 := os.Stat(dest)

	if os.IsNotExist(err1) {
		fmt.Println("dest does not exist")
		os.Exit(1)
	}

	if err1 == nil {
		fmt.Println("dest file")
		fmt.Println("bytes: ", destInfo.Size())    // bytes
		fmt.Println("mod time: " + destInfo.ModTime().Format(time.DateTime)) // time.Time
		fmt.Println("is dir: ", destInfo.IsDir())   // bool
		fmt.Println("mode: ", destInfo.Mode())    // os.FileMode
		fmt.Println()
	}
	


	// file watch & update loop

	lastModTime := destInfo.ModTime()

	for {
		time.Sleep(loopDelay)
		currTime := time.Now().UnixMilli()

		stat, err := os.Stat(source)
		if err != nil {
			fmt.Println(err)
			continue
		}

		if stat.ModTime().After(lastModTime) {
			fmt.Printf("source file modified after dest")

			fmt.Println("copying source to dest")

			sourceFile, err := os.Open(source)

			if err != nil {
				fmt.Println(err.Error())
			}

			destinationFile, err := os.Create(dest)
			if err != nil {
				fmt.Println(err.Error())
			}

			destInfo, err := destinationFile.Stat()
			if err != nil {
				fmt.Println(err.Error())
			}

		    _, err = io.Copy(destinationFile, sourceFile)

			if err != nil {
				fmt.Println(err.Error())
			} else {
				lastModTime = destInfo.ModTime()
			}

			if sourceFile != nil {
				sourceFile.Close()
			}
			if destinationFile != nil {
				destinationFile.Close()
			}
		}
		

		elapsedTime := time.Now().UnixMilli() - currTime
		fmt.Println("elapsedTime ", elapsedTime, " ms")
	}

	// i want to take the paths of all the files and keep them sync'd

	// on file update - sync all other files
	// on file lost or undetected - try to sync with the last modified file

	// add code for persistence

	// read from a file to get the file paths
	// output logs 
}