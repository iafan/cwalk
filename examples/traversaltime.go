package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/iafan/cwalk"
)

var fileCount int32
var folderCount int32
var errorCount int32
var openFiles = make(chan string)
var fileSlice []*os.File

// This callback simply counts files and folders.
//
// Note that the callback function should be thread-safe
// (this is why we use "atomic.AddInt32()" function to increment counters).
func callback(path string, info os.FileInfo, err error) error {
	if err != nil {
		atomic.AddInt32(&errorCount, 1)
		return err
	} else {
		if info.IsDir() {
			atomic.AddInt32(&folderCount, 1)
		} else {
			atomic.AddInt32(&fileCount, 1)
		}
	}
	return nil
}

// This callback simply counts files and folders and also opens the file.
//
// Note that the callback function should be thread-safe
// (this is why we use "atomic.AddInt32()" function to increment counters).
func errorCallback(path string, info os.FileInfo, err error) error {
	if err != nil {
		atomic.AddInt32(&errorCount, 1)
		return err
	} else {
		if info.IsDir() {
			atomic.AddInt32(&folderCount, 1)
		} else {
			f, _ := os.Open(path)
			fileSlice = append(fileSlice, f)
			atomic.AddInt32(&fileCount, 1)
		}
	}
	return nil
}

func main() {
	if len(os.Args) < 2 || os.Args[1] == "" {
		fmt.Println("Usage: traversaltime <directory-to-scan>")
		os.Exit(0)
	}
	dir := os.Args[1]

	// run the concurrent version

	folderCount = 0
	fileCount = 0
	errorCount = 0

	fmt.Print("Running concurrent version... ")
	start := time.Now()

	err := cwalk.Walk(dir, callback)

	fmt.Printf("done in %s\n", time.Since(start))
	fmt.Printf("\t%d directories found\n", folderCount)
	fmt.Printf("\t%d files found\n", fileCount)
	fmt.Printf("\t%d errors detected by the callback\n", errorCount)

	if err != nil {
		fmt.Printf("\t%d errors returned by cwalk\n", len(err.(cwalk.WalkerErrorList).ErrorList))
		fmt.Printf("Error :\n%s\n", err.Error())
	}

	// run the standard (single-threaded) version

	folderCount = 0
	fileCount = 0
	errorCount = 0

	fmt.Print("Running standard version... ")
	start = time.Now()

	err = filepath.Walk(dir, callback)

	fmt.Printf("done in %s\n", time.Since(start))
	fmt.Printf("\t%d directories found\n", folderCount)
	fmt.Printf("\t%d files found\n", fileCount)
	fmt.Printf("\t%d errors detected by the callback\n", errorCount)

	if err != nil {
		fmt.Printf("Error : %s\n", err.Error())
	}

	// run the concurrent triggering errors version

	folderCount = 0
	fileCount = 0
	errorCount = 0

	fmt.Print("Running concurrent version to trigger errors...\n")
	fmt.Print("Errors are triggered by exceeding file limits by opening every file... ")

	start = time.Now()

	//start the open files func
	time.Sleep(1000)

	err = cwalk.Walk(dir, errorCallback)

	close(openFiles)

	fmt.Printf("done in %s\n", time.Since(start))
	fmt.Printf("\t%d directories found\n", folderCount)
	fmt.Printf("\t%d files found\n", fileCount)
	fmt.Printf("\t%d errors detected by the callback\n", errorCount)

	if err != nil {
		fmt.Printf("\t%d errors returned by cwalk\n", len(err.(cwalk.WalkerErrorList).ErrorList))
		fmt.Printf("Error :\n%s\n", err.Error())
	}
}
