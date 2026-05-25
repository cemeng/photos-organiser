# photos-organiser

Renamer needs to be run first.

## TL;DR

* Download pictures from camera / phones etc
* Run renamer
* Run organiser

## Renamer

After you download your pictures from your phone, they'd normally end up in the `/Pictures` directory.

First run the `renamer`: `renamer -src=source-dir/ -dest=destination-dir/`

The command will process the files in the source folder by doing the following:
* Rename the file and move it to the destination folder
* Remove the original file into `/processed` folder

`go run ./cmd/renamer/ -src="~/Pictures/camera/2017/" -dest="/Volumes/Second MacMini HDD/Pictures/2017/"`

The `dest` folder is optional - if not supplied, it will use the `source` folder as destination.

## Importer

Importer is a TUI tool that combines renaming and organising into a single step. Use it instead of running renamer + organiser separately.

```
go run ./cmd/importer/ <source-directory>
```

For example:
```
go run ./cmd/importer/ ~/Desktop/iphone-staging/
```

The tool will:
1. Prompt you to confirm (or change) the destination directory — defaults to the parent of the source
2. Scan the source and show a summary of files grouped by destination month
3. Ask for confirmation before making any changes
4. Copy each file to `<dest>/YYYY/MM/YYYY-MM-DD-HH-mm-<original-name>.<ext>`
5. Move the original to a `processed/` subfolder inside the source
6. Write an `import-report-YYYY-MM-DD-HH-mm-SS.txt` to the destination when done

Files already matching the `YYYY-MM-DD-...` naming pattern are skipped. If a file with the same name already exists at the destination, a SHA256 comparison is done — identical files are silently skipped, different files are flagged as collisions in the report.

## Organiser

Organiser will *copy* pictures from a year folder to the month folders.
Organiser will create the month folders if they don't already exist.

*NOTE*: organiser will only copy the pics to the folders, you need to clean up
the copied files afterwards.

For example:
```
go run ./cmd/organiser/ -src="/Volumes/Second MacMini HDD/Pictures/2017/"
rm "/Volumes/Second MacMini HDD/Pictures/2017/*.*"
```
