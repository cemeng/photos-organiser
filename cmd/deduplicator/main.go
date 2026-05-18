package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
)

type FileInfo struct {
	Path string
	Size int64
	Hash string
}

// worker processes files from the paths channel and sends results to the results channel
func worker(id int, paths <-chan string, results chan<- FileInfo, wg *sync.WaitGroup) {
	defer wg.Done()
	for path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			fmt.Printf("Warning: Could not stat %s: %v\n", path, err)
			continue
		}

		hash, err := calculateFileHash(path)
		if err != nil {
			fmt.Printf("Warning: Could not process %s: %v\n", path, err)
			continue
		}

		results <- FileInfo{
			Path: path,
			Size: info.Size(),
			Hash: hash,
		}
	}
}

func main() {
	srcPtr := flag.String("src", "", "Source directory to scan for duplicates")
	deletePtr := flag.Bool("delete", false, "Delete duplicate files, keeping only one copy")
	dryRunPtr := flag.Bool("dry-run", false, "Show which files would be deleted without actually deleting them")
	helpPtr := flag.Bool("help", false, "Show help message")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Deduplicator helps find duplicate files in a directory and its subdirectories.\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s -src [directory]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "The tool works by:\n")
		fmt.Fprintf(os.Stderr, "1. Walking through all files in the specified directory and subdirectories\n")
		fmt.Fprintf(os.Stderr, "2. Computing SHA256 hash of file contents to detect duplicates\n")
		fmt.Fprintf(os.Stderr, "3. Reporting groups of duplicate files\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *helpPtr {
		flag.Usage()
		return
	}

	if *srcPtr == "" {
		fmt.Println("Error: src directory is required")
		flag.Usage()
		os.Exit(1)
	}

	// Expand tilde in path if present
	if len(*srcPtr) > 0 && (*srcPtr)[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Printf("Error getting home directory: %v\n", err)
			os.Exit(1)
		}
		*srcPtr = filepath.Join(homeDir, (*srcPtr)[1:])
	}

	// Check if directory exists
	srcInfo, err := os.Stat(*srcPtr)
	if err != nil {
		fmt.Printf("Error accessing source directory: %v\n", err)
		os.Exit(1)
	}
	if !srcInfo.IsDir() {
		fmt.Printf("Error: %s is not a directory\n", *srcPtr)
		os.Exit(1)
	}

	// Calculate number of workers based on CPU cores
	numWorkers := runtime.NumCPU()

	// Create channels for coordination
	paths := make(chan string)
	results := make(chan FileInfo)

	// Create WaitGroup for workers
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker(i, paths, results, &wg)
	}

	// Counters for statistics
	var (
		numDirs  int64
		numFiles int64
		statsMu  sync.Mutex
	)

	// Start a goroutine to walk the directory
	go func() {
		err = filepath.Walk(*srcPtr, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				statsMu.Lock()
				numDirs++
				statsMu.Unlock()
				return nil
			}

			statsMu.Lock()
			numFiles++
			statsMu.Unlock()
			paths <- path
			return nil
		})

		if err != nil {
			fmt.Printf("Error walking through directory: %v\n", err)
			os.Exit(1)
		}
		close(paths)
	}()

	// Start a goroutine to wait for workers to finish and close results channel
	go func() {
		wg.Wait()
		close(results)
	}()

	// Map to store files by their hash
	filesByHash := make(map[string][]FileInfo)

	// Collect results
	for result := range results {
		filesByHash[result.Hash] = append(filesByHash[result.Hash], result)
	}

	// Process and print duplicate files
	duplicatesFound := false
	var duplicateGroups int
	var totalDuplicates int
	var totalBytesFreed int64

	for hash, files := range filesByHash {
		if len(files) > 1 {
			if !duplicatesFound {
				if *deletePtr {
					fmt.Println("Deleting duplicate files:")
				} else if *dryRunPtr {
					fmt.Println("The following files would be deleted:")
				} else {
					fmt.Println("Found duplicate files:")
				}
				duplicatesFound = true
			}

			duplicateGroups++
			totalDuplicates += len(files) - 1  // subtract 1 to count only the duplicates (not the original)

			// Sort files to ensure consistent selection of which file to keep
			// Keep the file with the shortest path (usually the one closest to root)
			sort.Slice(files, func(i, j int) bool {
				return len(files[i].Path) < len(files[j].Path)
			})

			fmt.Printf("\nDuplicate group (SHA256: %s):\n", hash[:8])
			// Always keep the first file (shortest path)
			fmt.Printf("Keeping: %s (size: %d bytes)\n", files[0].Path, files[0].Size)

			// Process duplicates (all but the first file)
			for _, file := range files[1:] {
				if *deletePtr || *dryRunPtr {
					fmt.Printf("Deleting: %s (size: %d bytes)\n", file.Path, file.Size)
					totalBytesFreed += file.Size
				} else {
					fmt.Printf("- %s (size: %d bytes)\n", file.Path, file.Size)
				}

				// Actually delete the file if -delete is true and not in dry run mode
				if *deletePtr && !*dryRunPtr {
					err := os.Remove(file.Path)
					if err != nil {
						fmt.Printf("Error deleting %s: %v\n", file.Path, err)
					}
				}
			}
		}
	}

	if !duplicatesFound {
		fmt.Println("No duplicate files found.")
	}

	// Print statistics
	fmt.Printf("\nProcessing Summary:\n")
	fmt.Printf("- Directories scanned: %d\n", numDirs)
	fmt.Printf("- Files processed: %d\n", numFiles)
	fmt.Printf("- Duplicate groups found: %d\n", duplicateGroups)
	fmt.Printf("- Total duplicates found: %d\n", totalDuplicates)
	if *deletePtr || *dryRunPtr {
		fmt.Printf("- Total space that %s be freed: %s\n", 
			map[bool]string{true: "will", false: "would"}[*deletePtr],
			formatBytes(totalBytesFreed))
}

func calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// formatBytes converts bytes to a human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
