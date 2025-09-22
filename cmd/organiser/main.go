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
	// srcDirectory := "/Volumes/Second MacMini HDD/Pictures/2017/"
	var srcDirectory string
	flag.StringVar(&srcDirectory, "src", "", "source directory")
	flag.Parse()

	if srcDirectory == "" {
		log.Fatal("src argument is required, usage: organiser -src=xx/ ")
	}

	files, err := os.ReadDir(srcDirectory)
	if err != nil {
		log.Fatal(err)
	}
	// create month buckets
	for i := 1; i <= 12; i++ {
		m := fmt.Sprintf("%02d", i)
		err = createDirIfNotExist(srcDirectory + m + "/")
		if err != nil {
			log.Fatalf("Error creating directory %s", m)
		}
	}

	for _, f := range files {
		filename := f.Name()
		if f.IsDir() || filename == ".DS_Store" {
			continue
		}
		err := processFile(srcDirectory, filename)
		if err != nil {
			log.Fatalf("Error processing file %s: %s", filename, err)
		}
	}
}

func processFile(srcDirectory, filename string) error {
	// Filename has to be on the form of: 2018-07-19-13-18-s8fx.JPG
	r := strings.Split(filename, "-")
	month := r[1]

	destDirectory := fmt.Sprintf("%s%s/", srcDirectory, month)
	cmd := exec.Command("cp", "-a", srcDirectory+filename, destDirectory)
	err := cmd.Run()
	fmt.Printf("copying to %s\n", destDirectory+filename)
	if err != nil {
		errors.Wrap(err, "Error copying file")
	}

	// Move source file to processed directory
	newpath := filepath.Join(srcDirectory, "processed")
	err = os.MkdirAll(newpath, os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "error creating processed directory")
	}
	// move source file to processed
	cmd = exec.Command("mv", srcDirectory+filename, srcDirectory+"/processed/"+filename)
	err = cmd.Run()
	if err != nil {
		fmt.Printf("Error moving source file to processed")
		return errors.Wrap(err, "Error moving source file")
	}

	return nil
}

func createDirIfNotExist(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return errors.Wrap(err, "Error creating directory")
		}
	}
	return nil
}
