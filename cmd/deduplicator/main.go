package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type FileInfo struct {
	Path string
	Size int64
	Hash string
}

func main() {
	srcPtr := flag.String("src", "", "Source directory to scan for duplicates")
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

	// Map to store files by their hash
	filesByHash := make(map[string][]FileInfo)

	// Walk through directory
	err = filepath.Walk(*srcPtr, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Calculate file hash
		hash, err := calculateFileHash(path)
		if err != nil {
			fmt.Printf("Warning: Could not process %s: %v\n", path, err)
			return nil
		}

		// Store file info
		fileInfo := FileInfo{
			Path: path,
			Size: info.Size(),
			Hash: hash,
		}
		filesByHash[hash] = append(filesByHash[hash], fileInfo)

		return nil
	})

	if err != nil {
		fmt.Printf("Error walking through directory: %v\n", err)
		os.Exit(1)
	}

	// Print duplicate files
	duplicatesFound := false
	for hash, files := range filesByHash {
		if len(files) > 1 {
			if !duplicatesFound {
				fmt.Println("Found duplicate files:")
				duplicatesFound = true
			}
			fmt.Printf("\nDuplicate group (SHA256: %s):\n", hash[:8])
			for _, file := range files {
				fmt.Printf("- %s (size: %d bytes)\n", file.Path, file.Size)
			}
		}
	}

	if !duplicatesFound {
		fmt.Println("No duplicate files found.")
	}
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
