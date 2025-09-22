package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/mknote"
)

func main() {
	var srcDirectory string
	var destDirectory string
	flag.StringVar(&srcDirectory, "src", "", "source directory")
	flag.StringVar(&destDirectory, "dest", "", "destination directory")
	flag.Parse()

	if srcDirectory == "" {
		log.Fatal("src argument is required, dest argument is optional, usage: renamer -src=xx/ -dest=xx/")
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
		err := processFile(srcDirectory, destDirectory, filename)
		if err != nil {
			log.Fatalf("Error processing file %s: %s", filename, err)
		}
	}
}

func processFile(srcDirectory, destDirectory, fname string) error {
	result := strings.Split(fname, ".")
	filename := result[0]
	extension := result[1]

	var destFilename string
	var err error
	switch extension {
	case "JPG", "jpg", "HEIC":
		destFilename, err = filenameFromExif(srcDirectory, filename, extension)
		if err != nil {
			// Getting filename from exif fails, use file attribute as failback
			destFilename, err = filenameFromAttribute(srcDirectory, filename, extension)
			if err != nil {
				return errors.Wrap(err, "Error getting filename from exif and attribute")
			}
		}
	case "MOV", "mov", "PNG", "png", "MP4", "mp4", "3gp":
		destFilename, err = filenameFromAttribute(srcDirectory, filename, extension)
		if err != nil {
			return errors.Wrap(err, "Error getting filename from attribute")
		}
	default:
		fmt.Printf("Ignoring file with unsupported extension: %s\n", fname)
		// return errors.New("Cannot handle file with extension " + extension)
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

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"

func randomSuffix(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
