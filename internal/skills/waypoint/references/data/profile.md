# Profile document

The profile is written as a **profile document** — one JSON object, the same
shape `profile show --json` emits. Read it, edit it, write it back. One idiom
for every profile change.

## Read

```bash
waypoint profile show --json
```

The document carries structured entries. Their `description` fields are the
depth — pull from them for resume bullets, STAR answers, and cover letters.

## Write

1. **See the shape** — `waypoint profile schema --json` prints the empty
   template. Every key in it is writable; keys absent from your document stay
   unchanged (patch semantics).
2. **Write the document with your file tool** — platform temp dir, never a
   shell heredoc. The file is read directly, so nothing is shell-interpreted:
   shell-safe by construction.
3. **Apply it** — `waypoint profile set --file <path>`. Use `-` for stdin:
   `waypoint profile show --json | waypoint profile set --file -`.
4. **Verify** — `waypoint profile show --json` round-trips the fields you set.

Unknown keys are rejected — a typo is a hard error, never a silent drop.

**Done when**: every profile change you intend goes through a document written
with your file tool and passed to `set --file`. No inline JSON flags.

## Rules

- **Dates** are `YYYY-MM` (or `YYYY` when the month is unknown); an empty
  `end` means "present".
- **experience** entries: `title` required; `description` is the depth (scope,
  achievements, impact).
- **education** entries: `institution` required; `description` is the depth
  (GPA, focus areas).
- **salaryFloor** is `[{region, amount}]` — region required, amount a positive
  number. Currency is derived, never written.
- **seniority** is derived from experience once experience carries a year
  signal — don't write it unless no experience exists yet.

## Seed

A resume (PDF) is the usual source for experience and education — `read`
[data/pdf-extract](pdf-extract.md), then write the entries through the flow
above.
