# PDF Text Extraction

If `pdftotext` (poppler) available, extract text from PDFs — job postings, company dossiers, research papers.

## Usage

```bash
# extract to stdout
pdftotext <file.pdf> -

# first N lines
pdftotext <file.pdf> - | head -200

# extract to file
pdftotext <file.pdf> /tmp/extracted.txt
```

Then pipe into waypoint:
```bash
# add notes from PDF
waypoint jobs update <id> --notes "$(pdftotext file.pdf - | head -100)"

# or save as artifact
pdftotext file.pdf /tmp/job-posting.txt
waypoint artifacts add --skill resume-optimizer --title "Job Posting" -f /tmp/job-posting.txt --job <id>
```

## Handling remote PDFs

Exa can't fetch PDFs (403s common). Download first:
```bash
curl -sL -o /tmp/dl.pdf "<url>"
pdftotext /tmp/dl.pdf - | head -200
```
