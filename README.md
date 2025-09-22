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

`go run cmd/renamer/main.go -src="~/Pictures/camera/2017/" -dest="/Volumes/Second MacMini HDD/Pictures/2017/"`

The `dest` folder is optional - if not supplied, it will use the `source` folder as destination.

## Organiser

Organiser will *copy* pictures from a year folder to the month folders.
Organiser will create the month folders if they don't already exist.

*NOTE*: organiser will only copy the pics to the folders, you need to clean up
the copied files afterwards.

For example:
```
go run cmd/organiser/main.go -src="/Volumes/Second MacMini HDD/Pictures/2017/"
rm "/Volumes/Second MacMini HDD/Pictures/2017/*.*"
```
