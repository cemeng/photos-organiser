package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTimeToFilename(t *testing.T) {
	// Create a fixed time for testing
	testTime := time.Date(2023, time.January, 15, 14, 30, 45, 0, time.UTC)

	tests := []struct {
		name      string
		time      time.Time
		extension string
		want      string
	}{
		{
			name:      "jpg extension",
			time:      testTime,
			extension: "jpg",
			want:      "2023-01-15-14-30-45-", // We'll check the random suffix separately
		},
		{
			name:      "PNG extension",
			time:      testTime,
			extension: "PNG",
			want:      "2023-01-15-14-30-45-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := timeToFilename(tt.time, tt.extension)
			if !strings.HasPrefix(got, tt.want) {
				t.Errorf("timeToFilename() = %v, want prefix %v", got, tt.want)
			}
			// Check that the random suffix is present and correct length
			parts := strings.Split(got, "-")
			suffix := strings.Split(parts[len(parts)-1], ".")[0]
			if len(suffix) != 4 {
				t.Errorf("Random suffix length = %d, want 4", len(suffix))
			}
			// Check file extension
			if !strings.HasSuffix(got, "."+tt.extension) {
				t.Errorf("Wrong file extension in %v, want .%v", got, tt.extension)
			}
		})
	}
}

func TestFilenameFromAttribute(t *testing.T) {
	// Get the current working directory which contains gopher-stand.jpg
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// Add trailing slash to match the expected format
	dir = dir + "/"

	got, err := filenameFromAttribute(dir, "gopher-stand", "jpg")
	if err != nil {
		t.Fatalf("filenameFromAttribute() error = %v", err)
	}

	// Get the file's actual modification time
	fileInfo, err := os.Stat(dir + "gopher-stand.jpg")
	if err != nil {
		t.Fatal(err)
	}
	modTime := fileInfo.ModTime()

	// Format the expected time string
	want := fmt.Sprintf("%d-%02d-%02d-%02d-%02d-%02d-",
		modTime.Year(), modTime.Month(), modTime.Day(),
		modTime.Hour(), modTime.Minute(), modTime.Second())

	if !strings.HasPrefix(got, want) {
		t.Errorf("filenameFromAttribute() = %v, want prefix %v", got, want)
	}

	// Check file extension
	if !strings.HasSuffix(got, ".jpg") {
		t.Errorf("Wrong file extension in %v, want .jpg", got)
	}
}

func TestProcessFile(t *testing.T) {
	// Create temporary destination directory
	destDir, err := os.MkdirTemp("", "photo-test-dest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(destDir)

	// Get the current working directory which contains gopher-stand.jpg
	srcDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// Add trailing slashes to match the expected format
	srcDir = srcDir + "/"
	destDir = destDir + "/"

	// Create a backup of gopher-stand.jpg
	originalBytes, err := os.ReadFile(filepath.Join(srcDir, "gopher-stand.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	// Defer to restore of the test file
	defer func() {
		err := os.WriteFile(filepath.Join(srcDir, "gopher-stand.jpg"), originalBytes, 0644)
		if err != nil {
			t.Fatal(err)
		}
	}()

	// Process the test file
	err = processFile(srcDir, destDir, "gopher-stand.jpg")
	if err != nil {
		t.Fatalf("processFile() error = %v", err)
	}

	// Verify the file was processed correctly
	files, err := os.ReadDir(destDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("Expected 1 file in destination, got %d", len(files))
	}

	// Check that the source file was moved to processed directory
	processedPath := filepath.Join(srcDir, "processed", "gopher-stand.jpg")
	if _, err := os.Stat(processedPath); os.IsNotExist(err) {
		t.Error("Source file was not moved to processed directory")
	}

	// Clean up the processed directory after test
	err = os.RemoveAll(filepath.Join(srcDir, "processed"))
	if err != nil {
		t.Fatal(err)
	}
}
