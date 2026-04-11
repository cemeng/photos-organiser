# AGENTS.md

Guidance for Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Run all tests
go test ./...

# Run tests for a specific command
go test ./cmd/renamer/...

# Run a single test
go test ./cmd/renamer/... -run TestProcessFile

# Run any command directly
go run cmd/renamer/main.go -src=<source_dir>/ [-dest=<dest_dir>/] [-dry-run]
go run cmd/organiser/main.go -src=<source_dir>/ [-dry-run]
go run cmd/deduplicator/main.go -src=<dir>

# Build a binary
go build -o renamer ./cmd/renamer/
```

## Architecture

Three independent CLI tools under `cmd/`, each in its own `package main`:

- **renamer** (`cmd/renamer/`) — Processes raw photo/video files: reads EXIF data (JPG, HEIC) or file modification time (MOV, PNG, MP4, 3gp) to rename files to `YYYY-MM-DD-HH-mm-SS-xxxx.ext` format. Copies renamed file to destination and moves original to a `processed/` subdirectory in the source. Files already matching the output pattern are skipped.

- **organiser** (`cmd/organiser/`) — Takes a year folder of already-renamed files and copies them into month subfolders (`01/` through `12/`) based on the month embedded in the filename. Does not delete originals — caller must clean up.

- **deduplicator** (`cmd/deduplicator/`) — Walks a directory tree, computes SHA256 hashes of all files, and reports groups of duplicate files. Read-only; makes no changes.

## Workflow

Run tools in sequence: `renamer` → `organiser`. Renamer output (the `YYYY-MM-DD-...` naming convention) is the expected input format for organiser.

## Key details

- Source and destination directory paths **must** have a trailing slash.
- All three tools support `-dry-run` to preview actions without making changes.
- `cp -an` is used for copying (preserves attributes, no overwrite); this relies on the system `cp` command (macOS/Linux only).
- Tests in `cmd/renamer/` use `gopher-stand.jpg` as a fixture file and restore it after each test run.

## Code boundaries

Each tool in `cmd/` is self-contained in a single `main.go`. There are no shared packages. If you find yourself wanting to share code between tools, check with the user first — the current structure is intentional.

## Adding support for new file types

New extensions go in `cmd/renamer/main.go` inside `processFile()`. Follow the existing pattern:
- Use `filenameFromExif` for image formats that carry EXIF (JPG, HEIC).
- Use `filenameFromAttribute` for everything else (fallback to file modification time).

## Testing approach

- Tests live only in `cmd/renamer/` — the other tools have none yet.
- `gopher-stand.jpg` in `cmd/renamer/` is the test fixture; tests restore it after use via deferred `os.WriteFile`.
- When adding tests, follow this same pattern: use temp dirs for destinations, defer cleanup, restore any fixture files that get moved.
- Do not add mocks — tests use real files and the real filesystem.

## Safety rules

- Use `-dry-run` when testing CLI behavior against real directories — never run the tools against actual photo libraries without it.
- Never delete files — renamer moves originals to `processed/`; deduplicator is read-only; organiser copies only. Preserve this behavior.
- Do not introduce network calls, external services, or database dependencies.
