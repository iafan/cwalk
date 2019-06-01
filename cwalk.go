package cwalk

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// NumWorkers defines how many workers to run
// on each Walk() function invocation
var NumWorkers = runtime.GOMAXPROCS(0)

// BufferSize defines the size of the job buffer
var BufferSize = NumWorkers

// ErrNotDir indicates that the path, which is being passed
// to a walker function, does not point to a directory
var ErrNotDir = errors.New("Not a directory")

// A struct to store individual errors reported from each worker routine
type WalkerError struct {
	error error
	path  string
}

// A struct to store a list of errors reported from all worker routine
type WalkerErrorList struct {
	ErrorList []WalkerError
}

// Implement the error interface for WalkerError
func (we WalkerError) Error() string {
	return we.error.Error()
}

// Implement the error interface fo WalkerErrorList
func (wel WalkerErrorList) Error() string {
	if len(wel.ErrorList) > 0 {
		out := make([]string, len(wel.ErrorList))
		for i, err := range wel.ErrorList {
			out[i] = err.Error()
		}
		return strings.Join(out, "\n")
	}
	return ""
}

// Walker is constructed for each Walk() function invocation
type Walker struct {
	wg        sync.WaitGroup
	ewg       sync.WaitGroup // a separate wg for error collection
	jobs      chan string
	walkFunc  filepath.WalkFunc
	errors    chan WalkerError
	errorList WalkerErrorList // this is where we store the errors as we go
}

// the readDirNames function below was taken from the original
// implementation (see https://golang.org/src/path/filepath/path.go)
// but has sorting removed (sorting doesn't make sense
// in concurrent execution, anyway)

// readDirNames reads the directory named by dirname and returns
// a list of directory entries.
func readDirNames(dirname string) ([]string, error) {
	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	names, err := f.Readdirnames(-1)
	f.Close()
	if err != nil {
		return nil, err
	}
	return names, nil
}

// collectErrors processes any any errors passed via the error channel
// and stores them in the errorList
func (w *Walker) collectErrors() {
	defer w.ewg.Done()
	for err := range w.errors {
		w.errorList.ErrorList = append(w.errorList.ErrorList, err)
	}
}

// processPath processes one directory and adds
// its subdirectories to the queue for further processing
func (w *Walker) processPath(path string) error {
	defer w.wg.Done()

	names, err := readDirNames(path)
	if err != nil {
		return err
	}

	root := path
	for _, name := range names {
		path = filepath.Join(root, name)
		info, err := os.Lstat(path)
		err = w.walkFunc(path, info, err)

		if err == nil && info.IsDir() {
			w.addJob(path)
		}
		if err != nil && err != filepath.SkipDir {
			return err
		}
	}
	return nil
}

// addJob increments the job counter
// and pushes the path to the jobs channel
func (w *Walker) addJob(path string) {
	w.wg.Add(1)
	select {
	// try to push the job to the channel
	case w.jobs <- path: // ok
	default: // buffer overflow
		// process job synchronously
		err := w.processPath(path)
		if err != nil {
			w.errors <- WalkerError{
				error: err,
				path:  path,
			}
		}
	}
}

// worker processes all the jobs
// until the jobs channel is explicitly closed
func (w *Walker) worker() {
	for path := range w.jobs {
		err := w.processPath(path)
		if err != nil {
			w.errors <- WalkerError{
				error: err,
				path:  path,
			}
		}
	}

}

// Walk recursively descends into subdirectories,
// calling walkFn for each file or directory
// in the tree, including root directory.
// Walk does not follow symbolic links.
func (w *Walker) Walk(root string, walkFn filepath.WalkFunc) error {
	w.errors = make(chan WalkerError, BufferSize)
	w.jobs = make(chan string, BufferSize)
	w.walkFunc = walkFn

	w.ewg.Add(1) // a separate error waitgroup so we wait until all errors are reported before exiting
	go w.collectErrors()

	info, err := os.Lstat(root)
	if err == nil {
		err = w.walkFunc(root, info, err)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return ErrNotDir
	}

	// spawn workers
	for n := 1; n <= NumWorkers; n++ {
		go w.worker()
	}
	w.addJob(root)  // add root path as a first job
	w.wg.Wait()     // wait till all paths are processed
	close(w.jobs)   // signal workers to close
	close(w.errors) // signal errors to close
	w.ewg.Wait()    // wait for all errors to be collected

	if len(w.errorList.ErrorList) > 0 {
		return w.errorList
	}
	return nil
}

// Walk is a wrapper function for the Walker object
// that mimicks the behavior of filepath.Walk
func Walk(root string, walkFn filepath.WalkFunc) error {
	w := Walker{}
	return w.Walk(root, walkFn)
}
