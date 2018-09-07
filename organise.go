package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/mknote"
)

// arguments: src directory, destination directory
func main() {
	// TODO: doesn't work with mov files as they don't have exif info
	srcDirectory := "/Volumes/Second MacMini HDD/Pictures/2018/japan/"
	destDirectory := "/Volumes/Second MacMini HDD/Pictures/2018/japan/processed/"
	fname := "IMG_7343.JPG"

	result := strings.Split(fname, ".")
	filename := result[0]
	extension := result[1]

	var destFilename string
	var err error
	if extension == "JPG" || extension == "jpg" {
		destFilename, err = filenameFromExif(srcDirectory, filename, extension)
		if err != nil {
			log.Fatalf("Error getting filename %s", err)
		}
	}

	cmd := exec.Command("cp", "-a", srcDirectory+fname, destDirectory+destFilename) // cp -a preserves file attributes
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Error copying from %s to %s, err: %s", srcDirectory+fname, destDirectory+destFilename, err)
	}
	fmt.Printf("Copied to: %s\n", destDirectory+destFilename)
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

	// construct a new filename: yyyy-mm-dd-hh:mm:ss.JPG
	year, month, day := pictureTakenTime.Date()
	destFilename := fmt.Sprintf("%d-%02d-%d-%d:%d:%d.%s", year, month, day, pictureTakenTime.Hour(), pictureTakenTime.Minute(), pictureTakenTime.Second(), extension)

	fmt.Println("Converted: ", destFilename)
	return destFilename, nil
}
