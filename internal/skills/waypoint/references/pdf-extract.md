# PDF Text Extraction

Needs poppler (`pdftotext`, `pdftoppm`). If missing:

| OS | Install |
|-----|--------|
| macOS | `brew install poppler` |
| Ubuntu/Debian | `sudo apt install poppler-utils` |
| Fedora | `sudo dnf install poppler-utils` |
| Arch | `sudo pacman -S poppler` |
| Windows | `winget install poppler` / `choco install poppler` / `scoop install poppler` |

Check: `pdftotext -v 2>&1 | head -1`

## Step 1 — pdftotext

```bash
pdftotext <file.pdf> -              # stdout
pdftotext <file.pdf> - | head -200  # first N lines
pdftotext <file.pdf> /tmp/out.txt   # to file
```

**Done when**: text is readable and contains the job posting content.

If empty/garbled → step 2.

## Step 2 — PDF → image → vision model

If `pdftoppm` + vision model available:

```bash
pdftoppm -png -r 200 file.pdf /tmp/pdf-page              # all pages
pdftoppm -png -r 200 -f 1 -l 1 file.pdf /tmp/pdf-page   # page 1
pdftoppm -png -r 200 -f 1 -l 3 file.pdf /tmp/pdf-page   # pages 1-3
```

Send PNGs to vision model: "Extract all text. Include headings, lists, tables."

**Done when**: all pages extracted, text is readable.

## Step 3 — into waypoint

```bash
waypoint jobs update <id> --notes "$(pdftotext file.pdf - | head -100)"
pdftotext file.pdf /tmp/out.txt && waypoint artifacts add --skill resume-optimizer --title "Job Posting" -f /tmp/out.txt --job <id>
```

## Remote PDFs

Exa can't fetch PDFs. Download first:
```bash
curl -sL -o /tmp/dl.pdf "<url>" && pdftotext /tmp/dl.pdf - | head -200
```
