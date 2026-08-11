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

`<temp-dir>` below is `/tmp` on Unix, `$TEMP` on Windows.

## Step 1 — pdftotext

```
pdftotext <file.pdf> -                 # stdout
pdftotext <file.pdf> - | head -200     # first N lines (Unix pipe)
pdftotext <file.pdf> <temp-dir>/out.txt   # to file
```

**Done when**: text is readable and contains the job posting content.

If empty/garbled → step 2.

## Step 2 — PDF → image → vision model

If `pdftoppm` + vision model available:

```
pdftoppm -png -r 200 file.pdf <temp-dir>/pdf-page            # all pages
pdftoppm -png -r 200 -f 1 -l 1 file.pdf <temp-dir>/pdf-page  # page 1
pdftoppm -png -r 200 -f 1 -l 3 file.pdf <temp-dir>/pdf-page  # pages 1-3
```

Send PNGs to vision model: "Extract all text. Include headings, lists, tables."

**Done when**: all pages extracted, text is readable.

## Step 3 — into waypoint

Extracted text is shell-unsafe (long, may contain `$`/quotes/backticks), so write it to a file and pass it via file input — never `$(...)` or a pipe inline:

```
pdftotext <file.pdf> <temp-dir>/out.txt
waypoint jobs update <id> --notes-file <temp-dir>/out.txt
```
```
pdftotext <file.pdf> <temp-dir>/out.txt
waypoint artifacts add --skill resume-optimizer --title "Job Posting" -f <temp-dir>/out.txt --job <id>
```

`<temp-dir>` is `/tmp` on Unix, `$TEMP` on Windows.

## Remote PDFs

Exa can't fetch PDFs. Download to a temp path, then `pdftotext`:
```
curl -sL -o <temp-dir>/dl.pdf "<url>"
pdftotext <temp-dir>/dl.pdf -
```
