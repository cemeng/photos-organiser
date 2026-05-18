package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixtureJPG returns the path to gopher-stand.jpg in cmd/renamer/,
// and the original file bytes so tests can restore it via defer.
func fixtureJPG(t *testing.T) (path string, restore func()) {
	t.Helper()
	// cmd/importer/ is two levels below the module root; renamer is a sibling.
	abs, err := filepath.Abs("../renamer/gopher-stand.jpg")
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("cannot read fixture %s: %v", abs, err)
	}
	return abs, func() {
		if err := os.WriteFile(abs, original, 0644); err != nil {
			t.Fatalf("failed to restore fixture: %v", err)
		}
	}
}

// ── sanitizeBasename ──────────────────────────────────────────────────────────

func TestSanitizeBasename(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"IMG_1234", "img_1234"},
		{"My Photo 01", "my_photo_01"},
		{"file-name.backup", "file_name_backup"},
		{"HELLO WORLD", "hello_world"},
		{"__leading__trailing__", "leading_trailing"},
		{"abc", "abc"},
		{"A B  C", "a_b_c"},
	}
	for _, c := range cases {
		got := sanitizeBasename(c.in)
		if got != c.want {
			t.Errorf("sanitizeBasename(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ── buildDestFilename ─────────────────────────────────────────────────────────

func TestBuildDestFilename(t *testing.T) {
	t.Run("produces correct format", func(t *testing.T) {
		ts := time.Date(2024, 3, 15, 14, 22, 0, 0, time.UTC)
		got := buildDestFilename(ts, "IMG_1234", "JPG")
		want := "2024-03-15-14-22-img_1234.jpg"
		if got != want {
			t.Errorf("buildDestFilename() = %q, want %q", got, want)
		}
	})

	t.Run("lowercases extension", func(t *testing.T) {
		ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		got := buildDestFilename(ts, "photo", "HEIC")
		if !strings.HasSuffix(got, ".heic") {
			t.Errorf("expected lowercase extension, got %q", got)
		}
	})
}

// ── splitExtension ────────────────────────────────────────────────────────────

func TestSplitExtension(t *testing.T) {
	t.Run("normal file", func(t *testing.T) {
		ext, base, err := splitExtension("IMG_1234.JPG")
		if err != nil || ext != "JPG" || base != "IMG_1234" {
			t.Errorf("splitExtension() = %q %q %v", ext, base, err)
		}
	})
	t.Run("no extension returns error", func(t *testing.T) {
		_, _, err := splitExtension("noextension")
		if err == nil {
			t.Error("expected error for file with no extension")
		}
	})
	t.Run("file with multiple dots uses last as extension", func(t *testing.T) {
		ext, base, err := splitExtension("my.photo.JPG")
		if err != nil || ext != "JPG" || base != "my.photo" {
			t.Errorf("splitExtension() = %q %q %v", ext, base, err)
		}
	})
}

// ── DefaultDest ───────────────────────────────────────────────────────────────

func TestDefaultDest(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{"/Volumes/Photos/incoming/", "/Volumes/Photos/"},
		{"/Volumes/Photos/2024/camera/", "/Volumes/Photos/2024/"},
		{"/tmp/src/", "/tmp/"},
	}
	for _, c := range cases {
		got := DefaultDest(c.src)
		if got != c.want {
			t.Errorf("DefaultDest(%q) = %q, want %q", c.src, got, c.want)
		}
	}
}

// ── NormaliseDir ──────────────────────────────────────────────────────────────

func TestNormaliseDir(t *testing.T) {
	if got := NormaliseDir("/foo/bar"); got != "/foo/bar/" {
		t.Errorf("NormaliseDir() = %q, want trailing slash", got)
	}
	if got := NormaliseDir("/foo/bar/"); got != "/foo/bar/" {
		t.Errorf("NormaliseDir() double slash = %q", got)
	}
}

// ── ValidateDirectories ───────────────────────────────────────────────────────

func TestValidateDirectories(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()

	t.Run("valid dirs", func(t *testing.T) {
		if err := ValidateDirectories(src, dest); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("src == dest", func(t *testing.T) {
		if err := ValidateDirectories(src, src); err == nil {
			t.Error("expected error when src == dest")
		}
	})
	t.Run("nonexistent src", func(t *testing.T) {
		if err := ValidateDirectories("/does/not/exist", dest); err == nil {
			t.Error("expected error for nonexistent src")
		}
	})
}

// ── ScanDir ───────────────────────────────────────────────────────────────────

func TestScanDir(t *testing.T) {
	fixturePath, restoreFixture := fixtureJPG(t)
	defer restoreFixture()

	srcDir := t.TempDir() + "/"
	destDir := t.TempDir() + "/"

	// Copy fixture into src temp dir (don't move the original)
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "gopher-stand.jpg"), data, 0644); err != nil {
		t.Fatal(err)
	}
	// Add an unsupported file
	if err := os.WriteFile(filepath.Join(srcDir, "document.pdf"), []byte("pdf"), 0644); err != nil {
		t.Fatal(err)
	}
	// Add an already-processed file
	if err := os.WriteFile(filepath.Join(srcDir, "2024-03-15-14-22-img.jpg"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	plan, err := ScanDir(srcDir, destDir)
	if err != nil {
		t.Fatalf("ScanDir() error: %v", err)
	}

	var processable, unsupported, alreadyDone int
	for _, f := range plan.Files {
		switch f.Class {
		case ClassProcessable:
			processable++
		case ClassUnsupported:
			unsupported++
		case ClassAlreadyProcessed:
			alreadyDone++
		}
	}

	if processable != 1 {
		t.Errorf("want 1 processable, got %d", processable)
	}
	if unsupported != 1 {
		t.Errorf("want 1 unsupported, got %d", unsupported)
	}
	if alreadyDone != 1 {
		t.Errorf("want 1 already-processed, got %d", alreadyDone)
	}
}

// ── ExecuteOne (happy path) ───────────────────────────────────────────────────

func TestExecuteOne_HappyPath(t *testing.T) {
	fixturePath, restoreFixture := fixtureJPG(t)
	defer restoreFixture()

	srcDir := t.TempDir() + "/"
	destDir := t.TempDir() + "/"

	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	srcFile := filepath.Join(srcDir, "gopher-stand.jpg")
	if err := os.WriteFile(srcFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	plan, err := ScanDir(srcDir, destDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Files) == 0 {
		t.Fatal("scan produced no files")
	}

	fp := plan.Files[0]
	if fp.Class != ClassProcessable {
		t.Fatalf("expected ClassProcessable, got %v", fp.Class)
	}

	result := ExecuteOne(fp, srcDir)
	if result.Err != nil {
		t.Fatalf("ExecuteOne() error: %v", result.Err)
	}
	if !result.Succeeded {
		t.Error("expected Succeeded=true")
	}

	// Dest file should exist
	if _, err := os.Stat(fp.DestPath); err != nil {
		t.Errorf("dest file missing: %v", err)
	}

	// Original should be in processed/
	processedPath := filepath.Join(srcDir, "processed", "gopher-stand.jpg")
	if _, err := os.Stat(processedPath); err != nil {
		t.Errorf("original not moved to processed/: %v", err)
	}
}

// ── ExecuteOne (already processed — skip) ────────────────────────────────────

func TestExecuteOne_AlreadyProcessed(t *testing.T) {
	srcDir := t.TempDir() + "/"
	destDir := t.TempDir() + "/"

	name := "2024-03-15-14-22-img.jpg"
	if err := os.WriteFile(filepath.Join(srcDir, name), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	plan, err := ScanDir(srcDir, destDir)
	if err != nil {
		t.Fatal(err)
	}

	fp := plan.Files[0]
	if fp.Class != ClassAlreadyProcessed {
		t.Fatalf("expected ClassAlreadyProcessed, got %v", fp.Class)
	}

	result := ExecuteOne(fp, srcDir)
	if result.Succeeded {
		t.Error("already-processed file should not be marked Succeeded")
	}
	// Original file should be untouched
	if _, err := os.Stat(filepath.Join(srcDir, name)); err != nil {
		t.Errorf("original file should remain in src: %v", err)
	}
}

// ── ExecuteOne (unsupported extension — skip) ─────────────────────────────────

func TestExecuteOne_UnsupportedExtension(t *testing.T) {
	srcDir := t.TempDir() + "/"
	destDir := t.TempDir() + "/"

	if err := os.WriteFile(filepath.Join(srcDir, "document.pdf"), []byte("pdf"), 0644); err != nil {
		t.Fatal(err)
	}

	plan, err := ScanDir(srcDir, destDir)
	if err != nil {
		t.Fatal(err)
	}

	fp := plan.Files[0]
	if fp.Class != ClassUnsupported {
		t.Fatalf("expected ClassUnsupported, got %v", fp.Class)
	}
	if fp.SkipReason == "" {
		t.Error("expected a skip reason")
	}
}

// ── Collision detection ───────────────────────────────────────────────────────

func TestExecuteOne_CollisionDetection(t *testing.T) {
	fixturePath, restoreFixture := fixtureJPG(t)
	defer restoreFixture()

	srcDir := t.TempDir() + "/"
	destDir := t.TempDir() + "/"

	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}

	// Put the same file in src twice under different names but ensure they
	// map to the same dest filename (same timestamp, same basename).
	// Easiest: copy fixture, scan to get the dest path, pre-place a different
	// file at that dest path, then run ExecuteOne — it should detect collision.
	srcFile := filepath.Join(srcDir, "gopher-stand.jpg")
	if err := os.WriteFile(srcFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	plan, err := ScanDir(srcDir, destDir)
	if err != nil {
		t.Fatal(err)
	}
	fp := plan.Files[0]
	if fp.Class != ClassProcessable {
		t.Fatalf("expected ClassProcessable")
	}

	// Pre-place a *different* file at the dest path to simulate a collision
	if err := os.MkdirAll(filepath.Dir(fp.DestPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fp.DestPath, []byte("different content"), 0644); err != nil {
		t.Fatal(err)
	}

	result := ExecuteOne(fp, srcDir)
	if !result.Collision {
		t.Error("expected Collision=true when dest exists with different content")
	}
	if result.Succeeded {
		t.Error("colliding file should not be marked Succeeded")
	}
}

// ── FinaliseReport ────────────────────────────────────────────────────────────

func TestFinaliseReport(t *testing.T) {
	destDir := t.TempDir() + "/"

	report := &ImportReport{
		StartedAt:   time.Date(2024, 3, 15, 14, 22, 0, 0, time.UTC),
		Source:      "/src/",
		Destination: destDir,
		Results: []FileResult{
			{Plan: FilePlan{SourceName: "a.jpg", Class: ClassProcessable}, Succeeded: true},
			{Plan: FilePlan{SourceName: "b.pdf", Class: ClassUnsupported, SkipReason: "unsupported extension: .pdf"}},
		},
	}

	if err := FinaliseReport(report); err != nil {
		t.Fatalf("FinaliseReport() error: %v", err)
	}
	if report.ReportPath == "" {
		t.Error("expected ReportPath to be set")
	}
	if _, err := os.Stat(report.ReportPath); err != nil {
		t.Errorf("report file missing: %v", err)
	}

	contents, err := os.ReadFile(report.ReportPath)
	if err != nil {
		t.Fatal(err)
	}
	body := string(contents)
	for _, want := range []string{"Import Report", "Processed:", "Skipped:", "a.jpg", "b.pdf"} {
		if !strings.Contains(body, want) {
			t.Errorf("report missing %q\n---\n%s", want, body)
		}
	}
}
