package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/mknote"
)

var alreadyProcessedPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-\d{2}-\d{2}-`)

// FileClass is the result of classifying a source file.
type FileClass int

const (
	ClassProcessable FileClass = iota
	ClassAlreadyProcessed
	ClassUnsupported
)

// FilePlan describes what will happen to one source file.
type FilePlan struct {
	SourceName string    // original filename, e.g. IMG_1234.JPG
	SourcePath string    // full path to source file
	DestPath   string    // full destination path after rename
	DestDir    string    // YYYY/MM directory relative to dest root, e.g. "2024/03"
	Class      FileClass // how the file was classified
	SkipReason string    // set when Class != ClassProcessable
}

// FileResult records what actually happened during execution.
type FileResult struct {
	Plan      FilePlan
	Succeeded bool
	Collision bool // dest existed with different content
	Err       error
}

// ImportPlan is the full plan produced by ScanDir.
type ImportPlan struct {
	Source      string
	Destination string
	Files       []FilePlan
	// Grouped summary: destDir → count, for display
	Groups map[string]int
}

// ImportReport is the final result of Execute.
type ImportReport struct {
	StartedAt   time.Time
	Source      string
	Destination string
	Results     []FileResult
	ReportPath  string
}

func (r *ImportReport) Processed() int {
	n := 0
	for _, res := range r.Results {
		if res.Succeeded {
			n++
		}
	}
	return n
}

func (r *ImportReport) Skipped() []FileResult {
	var out []FileResult
	for _, res := range r.Results {
		if res.Plan.Class != ClassProcessable {
			out = append(out, res)
		}
	}
	return out
}

func (r *ImportReport) Collisions() []FileResult {
	var out []FileResult
	for _, res := range r.Results {
		if res.Collision {
			out = append(out, res)
		}
	}
	return out
}

func (r *ImportReport) Errors() []FileResult {
	var out []FileResult
	for _, res := range r.Results {
		if res.Err != nil {
			out = append(out, res)
		}
	}
	return out
}

// DefaultDest returns the parent directory of src.
// e.g. /Volumes/Photos/incoming/ → /Volumes/Photos/
func DefaultDest(src string) string {
	clean := filepath.Clean(src)
	return filepath.Dir(clean) + "/"
}

// NormaliseDir ensures a directory path has a trailing slash.
func NormaliseDir(path string) string {
	if path == "" {
		return path
	}
	if !strings.HasSuffix(path, "/") {
		return path + "/"
	}
	return path
}

// ValidateDirectories checks that src and dest exist, are directories, and are not equal.
func ValidateDirectories(src, dest string) error {
	if filepath.Clean(src) == filepath.Clean(dest) {
		return fmt.Errorf("source and destination must be different directories")
	}
	for _, dir := range []string{src, dest} {
		info, err := os.Stat(dir)
		if err != nil {
			return fmt.Errorf("cannot access %q: %w", dir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("%q is not a directory", dir)
		}
	}
	return nil
}

// ScanDir classifies all top-level files in src and builds an ImportPlan.
func ScanDir(src, dest string) (*ImportPlan, error) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return nil, fmt.Errorf("reading source directory: %w", err)
	}

	plan := &ImportPlan{
		Source:      src,
		Destination: dest,
		Groups:      make(map[string]int),
	}

	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == ".DS_Store" {
			continue
		}

		fp, err := classifyFile(src, dest, entry.Name())
		if err != nil {
			// non-fatal: treat as unsupported
			fp = FilePlan{
				SourceName: entry.Name(),
				SourcePath: filepath.Join(src, entry.Name()),
				Class:      ClassUnsupported,
				SkipReason: err.Error(),
			}
		}
		plan.Files = append(plan.Files, fp)
		if fp.Class == ClassProcessable {
			plan.Groups[fp.DestDir]++
		}
	}

	return plan, nil
}

func classifyFile(src, dest, name string) (FilePlan, error) {
	fp := FilePlan{
		SourceName: name,
		SourcePath: filepath.Join(src, name),
	}

	if alreadyProcessedPattern.MatchString(name) {
		fp.Class = ClassAlreadyProcessed
		fp.SkipReason = "already processed"
		return fp, nil
	}

	ext, base, err := splitExtension(name)
	if err != nil {
		fp.Class = ClassUnsupported
		fp.SkipReason = err.Error()
		return fp, nil
	}

	extLower := strings.ToLower(ext)
	var t time.Time

	switch extLower {
	case "jpg", "jpeg", "heic":
		t, err = timeFromExif(filepath.Join(src, name))
		if err != nil {
			// fallback to mod time
			t, err = timeFromModTime(filepath.Join(src, name))
			if err != nil {
				fp.Class = ClassUnsupported
				fp.SkipReason = fmt.Sprintf("could not determine date: %v", err)
				return fp, nil
			}
		}
	case "mov", "png", "mp4", "3gp":
		t, err = timeFromModTime(filepath.Join(src, name))
		if err != nil {
			fp.Class = ClassUnsupported
			fp.SkipReason = fmt.Sprintf("could not read mod time: %v", err)
			return fp, nil
		}
	default:
		fp.Class = ClassUnsupported
		fp.SkipReason = fmt.Sprintf("unsupported extension: .%s", ext)
		return fp, nil
	}

	destFilename := buildDestFilename(t, base, ext)
	destDir := fmt.Sprintf("%04d/%02d", t.Year(), t.Month())
	destPath := filepath.Join(dest, destDir, destFilename)

	fp.Class = ClassProcessable
	fp.DestDir = destDir
	fp.DestPath = destPath
	return fp, nil
}

// splitExtension returns (ext, base, error) for a filename.
// Rejects files with no extension or multiple dots in a way that is ambiguous.
func splitExtension(name string) (ext, base string, err error) {
	e := filepath.Ext(name) // includes the dot
	if e == "" {
		return "", "", fmt.Errorf("no file extension")
	}
	ext = strings.TrimPrefix(e, ".")
	base = strings.TrimSuffix(name, e)
	if base == "" {
		return "", "", fmt.Errorf("filename is empty after removing extension")
	}
	return ext, base, nil
}

// buildDestFilename produces YYYY-MM-DD-HH-mm-<sanitized-base>.<EXT>
func buildDestFilename(t time.Time, base, ext string) string {
	sanitized := sanitizeBasename(base)
	return fmt.Sprintf("%04d-%02d-%02d-%02d-%02d-%s.%s",
		t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(),
		sanitized, strings.ToUpper(ext))
}

// sanitizeBasename uppercases and replaces non-alphanumeric characters with _.
func sanitizeBasename(name string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	// collapse consecutive underscores
	result := regexp.MustCompile(`_+`).ReplaceAllString(b.String(), "_")
	// trim leading/trailing underscores
	return strings.Trim(result, "_")
}

func timeFromExif(path string) (time.Time, error) {
	f, err := os.Open(path)
	if err != nil {
		return time.Time{}, err
	}
	defer f.Close()

	exif.RegisterParsers(mknote.All...)
	data, err := exif.Decode(f)
	if err != nil {
		return time.Time{}, err
	}
	return data.DateTime()
}

func timeFromModTime(path string) (time.Time, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return fi.ModTime(), nil
}

// ExecuteOne processes a single FilePlan and returns the result.
// Call this for each file in plan.Files, then call FinaliseReport when all done.
func ExecuteOne(fp FilePlan, src string) FileResult {
	if fp.Class != ClassProcessable {
		return FileResult{Plan: fp}
	}
	return executeFile(fp, src)
}

// FinaliseReport writes the report file and attaches the path to the report.
func FinaliseReport(report *ImportReport) error {
	reportPath, err := writeReport(report)
	if err != nil {
		return fmt.Errorf("writing report: %w", err)
	}
	report.ReportPath = reportPath
	return nil
}

func executeFile(fp FilePlan, src string) FileResult {
	result := FileResult{Plan: fp}

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(fp.DestPath), 0755); err != nil {
		result.Err = fmt.Errorf("creating dest dir: %w", err)
		return result
	}

	// Copy preserving attributes; -n means no overwrite if dest exists.
	// On macOS, cp -n returns exit 1 when a file was skipped due to -n,
	// so we don't treat a non-zero exit as a fatal error here.
	exec.Command("cp", "-an", fp.SourcePath, fp.DestPath).Run() //nolint

	// Check whether dest exists at all — if not, the copy genuinely failed.
	if _, err := os.Stat(fp.DestPath); os.IsNotExist(err) {
		result.Err = fmt.Errorf("destination file missing after copy attempt")
		return result
	}

	// Dest exists — determine whether it's our copy or a pre-existing collision.
	collision, err := isCollision(fp.SourcePath, fp.DestPath)
	if err != nil {
		result.Err = fmt.Errorf("verifying copy: %w", err)
		return result
	}
	if collision {
		result.Collision = true
		return result
	}

	// Move original to processed/
	processedDir := filepath.Join(src, "processed")
	if err := os.MkdirAll(processedDir, 0755); err != nil {
		result.Err = fmt.Errorf("creating processed dir: %w", err)
		return result
	}
	dest := filepath.Join(processedDir, fp.SourceName)
	if err := os.Rename(fp.SourcePath, dest); err != nil {
		result.Err = fmt.Errorf("moving to processed: %w", err)
		return result
	}

	result.Succeeded = true
	return result
}

// isCollision returns true when dest exists but has different content from src.
// Caller must ensure dest exists before calling.
func isCollision(srcPath, destPath string) (bool, error) {
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return false, err
	}
	destInfo, err := os.Stat(destPath)
	if err != nil {
		return false, err
	}

	// Quick size check first
	if srcInfo.Size() != destInfo.Size() {
		return true, nil
	}

	// Same size: compare hashes to be sure
	srcHash, err := fileHash(srcPath)
	if err != nil {
		return false, err
	}
	destHash, err := fileHash(destPath)
	if err != nil {
		return false, err
	}
	return srcHash != destHash, nil
}

func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// writeReport writes the import report to the working directory and returns the path.
func writeReport(r *ImportReport) (string, error) {
	name := fmt.Sprintf("import-report-%s.txt",
		r.StartedAt.Format("2006-01-02-15-04-05"))
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	path := filepath.Join(wd, name)

	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	fmt.Fprintf(f, "Import Report — %s\n", r.StartedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(f, "Source:      %s\n", r.Source)
	fmt.Fprintf(f, "Destination: %s\n\n", r.Destination)

	fmt.Fprintf(f, "Summary\n")
	fmt.Fprintf(f, "  Processed:  %d\n", r.Processed())
	fmt.Fprintf(f, "  Skipped:    %d\n", len(r.Skipped()))
	fmt.Fprintf(f, "  Collisions: %d\n", len(r.Collisions()))
	fmt.Fprintf(f, "  Errors:     %d\n\n", len(r.Errors()))

	fmt.Fprintf(f, "Processed files\n")
	for _, res := range r.Results {
		if res.Succeeded {
			fmt.Fprintf(f, "  %s  →  %s\n", res.Plan.SourceName, res.Plan.DestPath)
		}
	}

	if len(r.Skipped()) > 0 {
		fmt.Fprintf(f, "\nSkipped files\n")
		for _, res := range r.Skipped() {
			fmt.Fprintf(f, "  %s   reason: %s\n", res.Plan.SourceName, res.Plan.SkipReason)
		}
	}

	if len(r.Collisions()) > 0 {
		fmt.Fprintf(f, "\nCollisions (not copied — different file exists at destination)\n")
		for _, res := range r.Collisions() {
			fmt.Fprintf(f, "  %s  →  %s\n", res.Plan.SourceName, res.Plan.DestPath)
		}
	}

	if len(r.Errors()) > 0 {
		fmt.Fprintf(f, "\nErrors\n")
		for _, res := range r.Errors() {
			fmt.Fprintf(f, "  %s   error: %v\n", res.Plan.SourceName, res.Plan.SkipReason)
		}
	}

	return path, nil
}
