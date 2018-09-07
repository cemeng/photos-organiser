package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/mknote"
)

func main() {
	srcDirectory := "/Volumes/Second MacMini HDD/Pictures/2018/japan/"
	destDirectory := "/Volumes/Second MacMini HDD/Pictures/2018/japan/processed/"
	rand.Seed(time.Now().UnixNano())

	files, err := ioutil.ReadDir(srcDirectory)
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
	if extension == "JPG" || extension == "jpg" {
		destFilename, err = filenameFromExif(srcDirectory, filename, extension)
		if err != nil {
			// Getting filename from exif fails, use file attribute as failback
			destFilename, err = filenameFromAttribute(srcDirectory, filename, extension)
			if err != nil {
				return errors.Wrap(err, "Error getting filename from exif and attribute")
			}
		}
	} else if extension == "MOV" || extension == "mov" || extension == "PNG" || extension == "png" {
		destFilename, err = filenameFromAttribute(srcDirectory, filename, extension)
		if err != nil {
			return errors.Wrap(err, "Error getting filename from attribute")
		}
	} else {
		return errors.New("Cannot handle file with extension " + extension)
	}

	cmd := exec.Command("cp", "-an", srcDirectory+fname, destDirectory+destFilename) // cp -a preserves file attributes
	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "Error copying")
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
	return fmt.Sprintf("%d-%02d-%02d-%02d-%02d-%s.%s", time.Year(), time.Month(), time.Day(), time.Hour(), time.Minute(), randomSuffix(4), extension)
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"

func randomSuffix(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
