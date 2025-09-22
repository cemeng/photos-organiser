package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

func main() {
	// Set custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Organiser - A tool to organize processed photos into monthly folders\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  %s -src=<source_dir>/ [-dry-run]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Description:\n")
		fmt.Fprintf(os.Stderr, "  Organiser takes processed photos (in YYYY-MM-DD-HH-mm-SS-xxxx.ext format)\n")
		fmt.Fprintf(os.Stderr, "  and organizes them into monthly folders (01/ through 12/).\n")
		fmt.Fprintf(os.Stderr, "  Files are copied to the appropriate month folder based on their names.\n")
		fmt.Fprintf(os.Stderr, "  Original files remain in the source directory.\n\n")
		fmt.Fprintf(os.Stderr, "Arguments:\n")
		fmt.Fprintf(os.Stderr, "  -src     Source directory containing the processed files (required)\n")
		fmt.Fprintf(os.Stderr, "  -dry-run Show what would be done without making any changes\n")
		fmt.Fprintf(os.Stderr, "  -help    Show this help message\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  organiser -src=~/Pictures/2023/\n")
		fmt.Fprintf(os.Stderr, "  organiser -src=~/Pictures/2023/ -dry-run\n\n")
		fmt.Fprintf(os.Stderr, "Note: Directory paths must end with a trailing slash (/)\n")
	}

	var srcDirectory string
	var dryRun bool
	flag.StringVar(&srcDirectory, "src", "", "source directory containing the processed files")
	flag.BoolVar(&dryRun, "dry-run", false, "show what would be done without making any changes")
	flag.Parse()

	if srcDirectory == "" && !containsHelpFlag() {
		fmt.Fprintf(os.Stderr, "Error: src argument is required\n\n")
		flag.Usage()
		os.Exit(1)
	}

	files, err := os.ReadDir(srcDirectory)
	if err != nil {
		log.Fatal(err)
	}
	// create month buckets
	for i := 1; i <= 12; i++ {
		m := fmt.Sprintf("%02d", i)
		err = createDirIfNotExist(srcDirectory+m+"/", dryRun)
		if err != nil {
			log.Fatalf("Error creating directory %s", m)
		}
	}

	for _, f := range files {
		filename := f.Name()
		if f.IsDir() || filename == ".DS_Store" {
			continue
		}

		// Check if file matches our expected format (YYYY-MM-DD-...)
		if !strings.HasPrefix(filename, "20") || len(strings.Split(filename, "-")) < 2 {
			if dryRun {
				fmt.Printf("[DRY-RUN] Skipping non-processed file: %s\n", filename)
			}
			continue
		}

		err := processFile(srcDirectory, filename, dryRun)
		if err != nil {
			log.Fatalf("Error processing file %s: %s", filename, err)
		}
	}
}

func processFile(srcDirectory, filename string, dryRun bool) error {
	// Filename has to be on the form of: 2018-07-19-13-18-s8fx.JPG
	r := strings.Split(filename, "-")
	if len(r) < 2 {
		return fmt.Errorf("invalid filename format: %s", filename)
	}
	month := r[1]

	destDirectory := fmt.Sprintf("%s%s/", srcDirectory, month)
	destPath := filepath.Join(destDirectory, filename)

	if dryRun {
		fmt.Printf("[DRY-RUN] Would copy:\n")
		fmt.Printf("  From: %s\n", filepath.Join(srcDirectory, filename))
		fmt.Printf("  To:   %s\n", destPath)
		fmt.Println("---")
		return nil
	}

	// Copy file to month directory
	cmd := exec.Command("cp", "-a", srcDirectory+filename, destDirectory)
	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, "Error copying file")
	}
	fmt.Printf("Copied to %s\n", destPath)

	return nil
}

func createDirIfNotExist(dir string, dryRun bool) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if dryRun {
			fmt.Printf("[DRY-RUN] Would create directory: %s\n", dir)
		} else {
			err = os.MkdirAll(dir, 0755)
			if err != nil {
				return errors.Wrap(err, "Error creating directory")
			}
		}
	}
	return nil
}

// containsHelpFlag checks if -h, -help, or --help is present in command line arguments
func containsHelpFlag() bool {
	for _, arg := range os.Args[1:] {
		if arg == "-h" || arg == "-help" || arg == "--help" {
			return true
		}
	}
	return false
}
