# Email Generator

Drafts job-search emails from job + profile data.

## Types

`application` · `followUp` · `thankYou` · `networking` · `referralRequest` · `offerAcceptance` · `rejectionResponse`

## Options

- **tone**: Formal | Casual | Creative | Concise
- **include salary**: bool
- **focus**: Skills | Experience | Education | Mixed
- **personal note/hook**: bool

## Steps

1. `waypoint jobs get <id>` — pull company, position, contact
2. `waypoint profile show --json` — pull name, skills, experience
3. Pick type + tone from user request
4. Draft from subject template + tone adjectives below
5. Rules: subject ≤78 chars · personal note ≤200 chars · always sign off

**Done when**: email has correct subject, swaps all placeholders, respects char limits, signs off.

## Subject templates

| Type | Subject |
|------|---------|
| application | `Application for {{position}} at {{company}}` |
| followUp | `Follow-Up: {{position}} Application` |
| thankYou | `Thank You - {{position}} Interview` |
| networking | `Connecting: {{position}} Interest at {{company}}` |
| referralRequest | `Referral Request: {{position}} at {{company}}` |
| offerAcceptance | `Offer Acceptance: {{position}} at {{company}}` |
| rejectionResponse | `Thank You — {{position}} at {{company}}` |

## Tone adjectives

- **Formal**: _proven, established, seasoned_. Closing: _Best regards / Sincerely_
- **Casual**: _passionate, enthusiastic, curious_. Closing: _Cheers / Best_
- **Creative**: _innovative, bold, dynamic_. Closing: _Looking forward / Onward_
- **Concise**: none — keep short. Closing: _Best_
