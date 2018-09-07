package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/mknote"
)

// arguments: src directory, destination directory
func main() {
	fname := "IMG_6145.MOV"

	f, err := os.Open(fname)
	if err != nil {
		log.Fatal(err)
	}

	exif.RegisterParsers(mknote.All...)

	pictureData, err := exif.Decode(f)
	if err != nil {
		log.Fatal(err)
	}

	pictureTakenTime, err := pictureData.DateTime()
	if err != nil {
		log.Fatal(err)
	}

	// construct a new filename: yyyy-mm-dd-hh:mm:ss.JPG
	year, month, day := pictureTakenTime.Date()
	destFilename := fmt.Sprintf("%d-%02d-%d-%d:%d:%d.%s", year, month, day, pictureTakenTime.Hour(), pictureTakenTime.Minute(), pictureTakenTime.Second(), "JPG")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Converted: ", destFilename)

	cmd := exec.Command("cp", "-a", fname, "processed/"+destFilename) // cp -a preserves file attributes
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Copied to: %s\n", "processed/"+destFilename)

	// use cp -a - which preserves the file attributes
}
