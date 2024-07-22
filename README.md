# remarkable2-pdf-generator

## Overview

This tool helps automate the process of downloading your notebooks as PDFs from your Remarkable2's web UI.
It will skip over any notebooks that haven't been changed since the last use of this tool.

Make sure to enable the web UI on your rm2 ([Guide](#enabling-the-web-ui))!

## Downloaded Files Structure

The location of the backups is set using the `-backupsDir` flag.

```
.
├── .last_modified      # used internally to determine whether to re-download a notebook
├── .backup.logs        # location of output logs if the -l flag is set
│
└── ... Your Remarkable Files ...
```

## Tool Usage

### Compile the tool
`go install ./`

The executable will the compiled to `${GOPATH}/bin/` or `go/bin/`

### Example usage
`./remarkable2-pdf-generator -v -l -backupsDir mybackup/ -i foo -i bar -e baz`

### Tool help
`./remarkable2-pdf-generator -h`

## Enabling the Web UI

1. Open the Menu tab from the home screen of your rm2.
2. Go to Settings > Storage.
3. Enable "USB web interface".