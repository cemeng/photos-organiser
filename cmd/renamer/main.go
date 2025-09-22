package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/mknote"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"

var (
	processedFilePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-\d{2}-\d{2}-\d{2}-[a-zA-Z0-9]{4}\.[a-zA-Z0-9]+$`)
)

func main() {
	// Set custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Renamer - A tool to organize photos and videos by their creation date\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  go run renamer/main.go -src=<source_dir>/ [-dest=<destination_dir>/] [-dry-run]\n\n")
		fmt.Fprintf(os.Stderr, "Description:\n")
		fmt.Fprintf(os.Stderr, "  Renamer processes photos and videos, organizing them by their creation date.\n")
		fmt.Fprintf(os.Stderr, "  For photos (JPG, HEIC), it uses EXIF data to get the creation date.\n")
		fmt.Fprintf(os.Stderr, "  For videos and other files (MOV, PNG, MP4, 3gp), it uses file modification time.\n")
		fmt.Fprintf(os.Stderr, "  Files are renamed to: YYYY-MM-DD-HH-mm-SS-xxxx.ext format\n")
		fmt.Fprintf(os.Stderr, "  where xxxx is a random suffix to prevent naming conflicts.\n\n")
		fmt.Fprintf(os.Stderr, "Arguments:\n")
		fmt.Fprintf(os.Stderr, "  -src    Source directory containing the files to process (required)\n")
		fmt.Fprintf(os.Stderr, "  -dest   Destination directory for processed files (optional, defaults to source)\n")
		fmt.Fprintf(os.Stderr, "  -dry-run Show what would be done without making any changes\n")
		fmt.Fprintf(os.Stderr, "  -help   Show this help message\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  renamer -src=~/Photos/\n")
		fmt.Fprintf(os.Stderr, "  renamer -src=~/Photos/ -dest=~/Organized/\n")
		fmt.Fprintf(os.Stderr, "  renamer -src=~/Photos/ -dry-run\n\n")
		fmt.Fprintf(os.Stderr, "Note: Directory paths must end with a trailing slash (/)\n")
	}

	var srcDirectory string
	var destDirectory string
	var dryRun bool
	flag.StringVar(&srcDirectory, "src", "", "source directory containing the files to process")
	flag.StringVar(&destDirectory, "dest", "", "destination directory for processed files")
	flag.BoolVar(&dryRun, "dry-run", false, "show what would be done without making actual changes")
	flag.Parse()

	if srcDirectory == "" && !containsHelpFlag() {
		fmt.Fprintf(os.Stderr, "Error: src argument is required\n\n")
		flag.Usage()
		os.Exit(1)
	}

	// Expand tilde to home directory in both source and destination paths
	var err error
	srcDirectory, err = expandTilde(srcDirectory)
	if err != nil {
		log.Fatal(err)
	}
	if destDirectory != "" {
		destDirectory, err = expandTilde(destDirectory)
		if err != nil {
			log.Fatal(err)
		}
	}

	// By default the destination directory will be the same as source directory if it is not supplied via dest argument
	if destDirectory == "" {
		destDirectory = srcDirectory
	}

	if srcDirectory[len(srcDirectory)-1:] != "/" || destDirectory[len(destDirectory)-1:] != "/" {
		log.Fatal("src and dest directories need a trailing slash, e.g: -src=directory/ not -src=directory")
	}

	files, err := os.ReadDir(srcDirectory)
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		filename := f.Name()
		if f.IsDir() || filename == ".DS_Store" {
			continue
		}

		// Check if file is already in our processed format
		if processedFilePattern.MatchString(filename) {
			if dryRun {
				fmt.Printf("[DRY-RUN] Skipping already processed file: %s\n", filename)
			}
			continue
		}

		err := processFile(srcDirectory, destDirectory, filename, dryRun)
		if err != nil {
			log.Fatalf("Error processing file %s: %s", filename, err)
		}
	}
}

func processFile(srcDirectory, destDirectory, fname string, dryRun bool) error {
	result := strings.Split(fname, ".")
	if len(result) != 2 {
		fmt.Printf("Ignoring file without extension: %s\n", fname)
		return nil
	}
	filename := result[0]
	extension := result[1]

	var destFilename string
	var err error
	if extension == "JPG" || extension == "jpg" || extension == "HEIC" {
		destFilename, err = filenameFromExif(srcDirectory, filename, extension)
		if err != nil {
			// Getting filename from exif fails, use file attribute as failback
			destFilename, err = filenameFromAttribute(srcDirectory, filename, extension)
			if err != nil {
				return errors.Wrap(err, "Error getting filename from exif and attribute")
			}
		}
	} else if extension == "MOV" || extension == "mov" || extension == "PNG" || extension == "png" || extension == "MP4" || extension == "mp4" || extension == "3gp" {
		destFilename, err = filenameFromAttribute(srcDirectory, filename, extension)
		if err != nil {
			return errors.Wrap(err, "Error getting filename from attribute")
		}
	} else {
		fmt.Printf("Ignoring file with unsupported extension: %s\n", fname)
		return nil
	}

	if dryRun {
		fmt.Printf("[DRY-RUN] Would rename:\n")
		fmt.Printf("  Source: %s\n", filepath.Join(srcDirectory, fname))
		fmt.Printf("  Destination: %s\n", filepath.Join(destDirectory, destFilename))
		fmt.Printf("  Then move source to: %s\n", filepath.Join(srcDirectory, "processed", fname))
		fmt.Println("---")
		return nil
	}

	// Copy the file to destination
	srcFile, err := os.Open(srcDirectory + fname)
	if err != nil {
		return errors.Wrap(err, "Error opening source file")
	}
	defer srcFile.Close()

	destFile, err := os.OpenFile(destDirectory+destFilename, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Error creating destination file %s", destDirectory+destFilename))
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return errors.Wrap(err, "Error copying file")
	}

	// move source file to processed directory
	newpath := filepath.Join(srcDirectory, "processed")
	err = os.MkdirAll(newpath, os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "error creating processed directory")
	}

	// move source file to processed using os.Rename
	err = os.Rename(srcDirectory+fname, filepath.Join(srcDirectory, "processed", fname))
	if err != nil {
		return errors.Wrap(err, "Error moving source file to processed")
	}

	fmt.Printf("%s processed\n", destDirectory+destFilename)
	return nil
}

func filenameFromAttribute(srcDirectory, filename, extension string) (string, error) {
	fullFilepath := srcDirectory + filename + "." + extension
	_, err := os.Open(fullFilepath)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(fullFilepath)
	if err != nil {
		return "", err
	}
	modifiedTime := fi.ModTime()
	return timeToFilename(modifiedTime, extension), nil
}

func filenameFromExif(srcDirectory, filename, extension string) (string, error) {
	f, err := os.Open(srcDirectory + filename + "." + extension)
	if err != nil {
		return "", err
	}

	exif.RegisterParsers(mknote.All...)

	pictureData, err := exif.Decode(f)
	if err != nil {
		return "", err
	}

	pictureTakenTime, err := pictureData.DateTime()
	if err != nil {
		return "", err
	}

	return timeToFilename(pictureTakenTime, extension), nil
}

func timeToFilename(time time.Time, extension string) string {
	return fmt.Sprintf("%d-%02d-%02d-%02d-%02d-%02d-%s.%s", time.Year(), time.Month(), time.Day(), time.Hour(), time.Minute(), time.Second(), randomSuffix(4), extension)
}

// expandTilde replaces ~ with the user's home directory
func expandTilde(path string) (string, error) {
	if path == "" {
		return path, nil
	}

	if path[0] != '~' {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrap(err, "could not get user home directory")
	}

	if len(path) == 1 {
		return home, nil
	}

	if path[1] == '/' {
		// Preserve trailing slash if it exists
		hasTrailingSlash := path[len(path)-1] == '/'
		expanded := filepath.Join(home, path[2:])
		if hasTrailingSlash {
			expanded += "/"
		}
		return expanded, nil
	}

	return path, nil
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

func randomSuffix(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
