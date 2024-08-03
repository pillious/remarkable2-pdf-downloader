# remarkable2-pdf-downloader

## Overview

This tool helps automate the process of downloading your notebooks as PDFs from your Remarkable2's web UI.
Customize the folders or notebooks you want to download and which ones to skip over. Any notebooks that haven't been changed since the last use of the tool will be skipped.

## Requirements:

1. Have Golang installed.
2. Make sure to enable the web UI on your Rm2 ([Guide](#enabling-the-web-ui))!
3. **Do not have duplicate names** in the same folder on your Remarakble (your PC filesystem can't handle that).

## Tool Usage

### Compile the tool
`go install ./`

The executable will the compiled to `${GOPATH}/bin/` or `go/bin/`

### Example usage
`./remarkable2-pdf-downloader -v -l -backupsDir mybackup/ -i foo/ -i bar/mynotes -e baz/quz/`

### Tool help
`./remarkable2-pdf-downloader -h`

## Downloaded Files Structure

The location of the backups is set using the `-backupsDir` flag.

```
.
├── .backup_info.json   # backup state info used internally
├── .backup.logs        # location of output logs if the -l flag is set
│
└── ... Your Remarkable Files ...
```

## Enabling the Web UI

1. Open the Menu tab from the home screen of your rm2.
2. Go to Settings > Storage.
3. Enable "USB web interface".