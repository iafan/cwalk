package main

import (
	"flag"
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

var followSymlinks bool
var processingTime time.Duration

// This callback simply counts files and folders.
//
// Note that the callback function should be thread-safe
// (this is why we use "atomic.AddInt32()" function to increment counters).
func callback(path string, info os.FileInfo, err error) error {
	if err != nil {
		atomic.AddInt32(&errorCount, 1)
	} else {
		if info.IsDir() {
			atomic.AddInt32(&folderCount, 1)
		} else {
			atomic.AddInt32(&fileCount, 1)
		}
	}
	if processingTime > 0 {
		time.Sleep(processingTime)
	}
	return nil
}

func init() {
	flag.BoolVar(&followSymlinks, "follow-symlinks", false, "When specified, directory symlinks will be processed and followed")
	flag.BoolVar(&followSymlinks, "f", false, "Shorthand for -follow-symlinks")

	flag.DurationVar(&processingTime, "file-processing-time", 0, "An artificial delay, for each file processed, to imitate actual work. Omitting this parameter means no delay. Example: 50ms")
	flag.DurationVar(&processingTime, "t", 0, "Shorthand for -file-processing-time")
}

func main() {
	flag.Parse()

	if len(flag.Args()) < 1 || flag.Args()[0] == "" {
		fmt.Println("Usage:")
		fmt.Println("  traversaltime [-f] [-t N] <directory-to-scan>")
		fmt.Println("Options:")
		flag.PrintDefaults()
		os.Exit(0)
	}
	dir := flag.Args()[0]
	fmt.Println("Directory:", dir)

	// run the concurrent version

	folderCount = 0
	fileCount = 0
	errorCount = 0

	start := time.Now()
	var err error

	if followSymlinks {
		fmt.Printf("Running a concurrent version that follows symlinks with %d workers and %s file processing time... ", cwalk.NumWorkers, processingTime)
		err = cwalk.WalkWithSymlinks(dir, callback)
	} else {
		fmt.Printf("Running a concurrent version that doesn't follow symlinks with %d workers and %s file processing time... ", cwalk.NumWorkers, processingTime)
		err = cwalk.Walk(dir, callback)
	}

	fmt.Printf("done in %s\n", time.Since(start))
	fmt.Printf("\t%d directories found\n", folderCount)
	fmt.Printf("\t%d files found\n", fileCount)
	fmt.Printf("\t%d errors found\n", errorCount)
	if err != nil {
		fmt.Printf("\nErrors: %s\n\n", err)
	}

	// run the standard (single-threaded) version

	folderCount = 0
	fileCount = 0
	errorCount = 0

	fmt.Printf("Running a standard version (single-threaded, doesn't follow symlinks) with %s file processing time... ", processingTime)
	start = time.Now()

	err = filepath.Walk(dir, callback)

	fmt.Printf("done in %s\n", time.Since(start))
	fmt.Printf("\t%d directories found\n", folderCount)
	fmt.Printf("\t%d files found\n", fileCount)
	fmt.Printf("\t%d errors found\n", errorCount)
	if err != nil {
		fmt.Printf("\nError: %s\n\n", err)
	}
}
