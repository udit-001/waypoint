# PDF Text Extraction

## 0. Prerequisites

Needs poppler (`pdftotext`, `pdftoppm`). If missing, install:

| OS | Command |
|-----|--------|
| macOS | `brew install poppler` |
| Ubuntu/Debian | `sudo apt install poppler-utils` |
| Fedora | `sudo dnf install poppler-utils` |
| Arch | `sudo pacman -S poppler` |
| Windows | `choco install poppler` or `scoop install poppler` |

Check: `pdftotext -v 2>&1 | head -1`

## 1. Try pdftotext first

```bash
# extract to stdout
pdftotext <file.pdf> -

# first N lines
pdftotext <file.pdf> - | head -200

# extract to file
pdftotext <file.pdf> /tmp/extracted.txt
```

Check if output is readable. If empty/garbled → step 2.

## 2. PDF to image → vision model

If `pdftoppm` (poppler) available and a vision model is accessible:

```bash
# convert pages to PNGs
pdftoppm -png -r 200 file.pdf /tmp/pdf-page

# produces /tmp/pdf-page-1.png, /tmp/pdf-page-2.png, etc.
```

Then send images to vision model with prompt like "Extract all text from this document. Include headings, lists, and tables."

For multi-page PDFs, process pages in batches or concatenate:
```bash
# single page
pdftoppm -png -r 200 -f 1 -l 1 file.pdf /tmp/pdf-page

# pages 1-3
pdftoppm -png -r 200 -f 1 -l 3 file.pdf /tmp/pdf-page
```

## 3. Pipe into waypoint

```bash
# add notes from extracted text
waypoint jobs update <id> --notes "$(pdftotext file.pdf - | head -100)"

# or save as artifact
pdftotext file.pdf /tmp/job-posting.txt
waypoint artifacts add --skill resume-optimizer --title "Job Posting" -f /tmp/job-posting.txt --job <id>
```

## Remote PDFs

Exa can't fetch PDFs (403s common). Download first:
```bash
curl -sL -o /tmp/dl.pdf "<url>"
pdftotext /tmp/dl.pdf - | head -200
```
