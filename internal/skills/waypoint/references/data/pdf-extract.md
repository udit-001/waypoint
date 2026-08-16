# PDF Text Extraction

`waypoint resume extract` is the single command for turning a resume PDF
into model-ready text. It keeps the contact info out of anything the model
sees. Use it whenever resume text is bound for an LLM: profile seeds,
resume-optimizer artifacts, cover letters, career summaries.

## The command

```
waypoint resume extract <resume.pdf>
```

Output is always JSON on stdout:

```
{
  "ok": true,
  "backend": "pdf_oxide",          # or "poppler" (fallback)
  "pages": 1,
  "chars": 2746,
  "redacted": true,
  "email_redacted": true,
  "phone_redacted": true,
  "spans": [ { "type": "phone", "start": 0, "end": 12 },
             { "type": "email", "start": 133, "end": 144 } ],
  "text": "…redacted text…"
}
```

- The `text` field is what you feed to the model. Emails and phone numbers
  are already replaced with `[REDACTED]`.
- The report fields are the proof: `redacted` must be `true` and the span
  list is the audit trail.

## Rules

1. **Redaction is the default.** `waypoint resume extract` redacts email and
   phone automatically. If the JSON shows `"redacted": false`, stop — check
   the input, do not send the text to a model.
2. **The text is shell-safe JSON.** Never pipe raw text, never `$(...)` —
   copy the `text` field (e.g. `jq -r .text`) or write it to a file when a
   command needs a file input.
3. **Persist via files.** The db/artifact commands take files:
   ```
   waypoint resume extract <resume.pdf> | jq -r .text > <tmp>/safe.txt
   waypoint jobs update <id> --notes-file <tmp>/safe.txt
   waypoint artifacts add --skill resume-optimizer --title "Job Posting" -f <tmp>/safe.txt --job <id>
   ```
   `<tmp>` is `/tmp` on Unix, `$TEMP` on Windows.
4. **`--no-redact` is for the user's eyes only.** Raw text (name, email,
   phone) must never reach the model. Only use it when the user asked to see
   the raw extraction.

## Backends — nothing to install

- **pdf_oxide** (default): the native library is fetched automatically on
  first use into the waypoint config dir (`<config>/lib/pdf_oxide/`),
  checksum-verified, cached. No system dependency.
- **poppler** (fallback): if the download is impossible (offline first run),
  `waypoint resume extract` falls back to `pdftotext` (needs poppler-utils).

Diagnose with:
```
waypoint resume doctor
```

## Remote PDFs

Exa can't fetch PDFs. Download to a temp path first, then extract:
```
curl -sL -o <tmp>/dl.pdf "<url>"
waypoint resume extract <tmp>/dl.pdf
```

## Done when

- `ok: true` and `backend` is one of `pdf_oxide` / `poppler`
- `redacted: true` (or the text is genuinely empty of contact info)
- The `text` field is in a file (for `--notes-file` / `-f`) or held as JSON
- Cross-check: the original PDF still holds the raw contact info; the
  extracted text does not