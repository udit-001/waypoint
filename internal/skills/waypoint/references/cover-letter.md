# Cover Letter

Cover letters in 4 tones.

## Options

- **tone**: Formal | Casual | Creative | Executive
- **length**: Short (250) | Medium (500) | Detailed (800 chars)
- **emphasize skill**: optional
- **include education**: bool

**Always**: contact header + signature. Swap `{{company}}` `{{position}}` `{{contactName}}`.

## Structures

- **Formal** — "Dear Hiring Manager": address-block → date → salutation → intro → experience → skills → education → closing → signature. Words: _proven, established, seasoned_.
- **Casual** — "Hi there": intro → highlights → fit → culture → closing. Words: _passionate, enthusiastic, curious_.
- **Creative** — "To the {{company}} Team": hook → story → skills → vision → CTA. Words: _innovative, bold, dynamic_.
- **Executive** — "Dear {{company}} Leadership Team": title-ref → strategic intro → leadership → vision → culture-fit → closing. Words: _strategic, visionary, transformational_.

## Steps

1. `waypoint jobs get <id>` — pull company, position, contact
2. `waypoint profile show --json` — pull name, skills, experience
3. Pick tone + length from user request
4. Draft following structure; enforce length cap

**Done when**: letter follows the tone's structure, respects length cap, swaps all placeholders.
