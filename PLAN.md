# Project Plan: `cmd/importer`

## Overview

A single interactive tool that replaces the manual renamer → organiser → rm workflow. It scans a source directory, renames files using EXIF/mod-time, copies them into a `YYYY/MM/` destination hierarchy, moves originals to `processed/`, and writes a report.

Built with [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) for an interactive terminal UI.

**Invocation:**
```
go run cmd/importer/main.go /path/to/source/
```

---

## Dependencies

```
github.com/charmbracelet/bubbletea   — TUI framework (Elm architecture)
github.com/charmbracelet/bubbles     — UI components: textinput, spinner, progress
github.com/charmbracelet/lipgloss    — terminal styling
```

---

## Architecture

Business logic (EXIF reading, filename generation, file copying) lives in pure Go functions, entirely separate from the TUI. The bubbletea model is a thin shell that calls these functions via `tea.Cmd` (runs in a goroutine, returns a message). This keeps all logic independently testable without the TUI.

```
main.go
  ├── model.go       — bubbletea Model, Update, View
  ├── importer.go    — pure business logic (scan, build plan, execute, report)
  └── main_test.go   — tests against importer.go only
```

---

## TUI flow — screens & states

The bubbletea model progresses through these states:

### Screen 1: Destination input (Phase 1)
- Uses `bubbles/textinput`, pre-filled with the default destination (parent of source)
- Enter to confirm, Ctrl+C/q to abort
- On confirm: validates both dirs, then transitions to Screen 2

### Screen 2: Scanning (Phase 2a)
- Uses `bubbles/spinner` while scanning runs as a `tea.Cmd` in the background
- On complete: transitions to Screen 3

### Screen 3: Plan confirmation (Phase 2b)
- Displays grouped summary:
  ```
  Found 47 processable files:
    → dest/2024/03/  (12 files)
    → dest/2024/04/  (23 files)
    → dest/2024/05/  (12 files)
  Skipped: 3 files (details in report)

  Proceed? [y/N]:
  ```
- y → transitions to Screen 4; n/Ctrl+C → abort

### Screen 4: Execution progress (Phase 4)
- Uses `bubbles/progress` bar: `Copying files... [=====>    ] 23/47`
- Shows current filename being processed
- Each file copy is a sequential `tea.Cmd`; on each completion the model updates and re-renders
- On complete: transitions to Screen 5

### Screen 5: Done (Phase 5)
- Final summary (processed / skipped / collisions / errors)
- Path to written report file
- Press any key to exit

---

## Phase 1 — Startup & validation

- Accept source dir as positional argument (no flag needed)
- Default destination: parent directory of source
- Validate both dirs exist and are directories
- Add trailing slash automatically if missing
- Guard: if source == destination, abort with a clear error

**Note:** Source scanning is flat (top-level files only, matching current behavior). Revisit if recursive scanning is needed later.

---

## Phase 2 — Scan & plan

- Walk source dir (flat), classify each file:
  - **Processable**: JPG, HEIC (EXIF), MOV, PNG, MP4, 3gp (mod time)
  - **Already processed**: matches `YYYY-MM-DD-HH-mm-*` pattern → skip, note in report
  - **Unsupported extension**: skip, note in report
  - **No extension / multiple dots**: skip, note in report
- Build a plan: map each file to its destination path
- Abort if user says no at confirmation screen

---

## Phase 3 — Filename format

New format: `YYYY-MM-DD-HH-mm-<original-basename>.<ext>`

- Drop seconds (less noise, human-readable)
- `<original-basename>` = source filename without extension, sanitized (lowercase, replace spaces/special chars with `_`)
- Extension preserved as-is from source

**Example:** `IMG_1234.JPG` taken at 2024-03-15 14:22 → `2024-03-15-14-22-img_1234.jpg`

**Collision edge case:** two files with identical timestamp + original name → `cp -an` would silently skip the second. Detection: after copy, compare dest file size against source; if mismatch, flag as collision in report.

**Deferred — bad date handling (revisit later):**
- Dates before 2000-01-01 → likely camera misconfiguration
- Future dates → likely wrong system clock
- EXIF vs mod-time discrepancy > 30 days → possible metadata corruption
- For now: these pass through silently

---

## Phase 4 — Execution

For each processable file:
1. Determine dest path: `<dest>/<YYYY>/<MM>/<new-filename>`
2. Create `<dest>/<YYYY>/<MM>/` if it doesn't exist (only dirs that are actually needed)
3. `cp -an` source → dest (preserves attributes, no overwrite)
4. Verify copy succeeded (check dest file exists)
5. `mv` original to `<source>/processed/<original-filename>`
6. Track result: success / collision-skipped / error

---

## Phase 5 — Report

Written to `<dest>/import-report-YYYY-MM-DD-HH-mm-SS.txt` after execution.

Contents:
```
Import Report — 2026-05-18 14:32:01
Source:      /Volumes/Photos/incoming/
Destination: /Volumes/Photos/

Summary
  Processed:  44
  Skipped:     3  (already processed: 1, unsupported: 2)
  Collisions:  0
  Errors:      0

Processed files
  IMG_1234.JPG  →  2024/03/2024-03-15-14-22-img_1234.jpg
  ...

Skipped files
  document.pdf   reason: unsupported extension
  ...
```

---

## Phase 6 — Tests

Tests cover `importer.go` (pure logic) only — no bubbletea model testing.
Following existing pattern (real files, real filesystem, temp dirs, no mocks).
Fixture: reuse `gopher-stand.jpg` from `cmd/renamer/` via relative path, same restore-after pattern.

| Test | Description |
|------|-------------|
| `TestBuildDestPath` | Unit test: EXIF date → correct `YYYY/MM/filename` |
| `TestBuildDestPath_ModTimeFallback` | No EXIF → uses mod time |
| `TestSanitizeFilename` | Spaces, special chars handled correctly |
| `TestScanDir` | Returns correct processable/skipped classification |
| `TestProcessFile_HappyPath` | JPG copied to correct dest, original moved to `processed/` |
| `TestProcessFile_AlreadyProcessed` | Skipped, not re-copied |
| `TestProcessFile_UnsupportedExtension` | Skipped, included in report |
| `TestProcessFile_CollisionDetection` | Second file with same dest name is flagged, not silently dropped |
| `TestWriteReport` | Report file written with correct content and path |

---

## Out of scope

- Recursive source scanning
- Bad/future date detection (deferred to Phase 3 note)
- Deleting source files (always manual)
- Modifying the existing `renamer` or `organiser` tools
