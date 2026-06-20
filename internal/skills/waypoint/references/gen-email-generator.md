# Email Generator

## Types
`application` · `followUp` · `thankYou` · `networking` · `referralRequest` · `offerAcceptance` · `rejectionResponse`

## Options
- **tone**: Formal | Casual | Creative | Concise
- **include salary**: bool
- **focus**: Skills | Experience | Education | Mixed
- **personal note/hook**: bool

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

## Tone adjectives & closings
- **Formal**: _proven, established, seasoned_ → _Best regards / Sincerely_
- **Casual**: _passionate, enthusiastic, curious_ → _Cheers / Best_
- **Creative**: _innovative, bold, dynamic_ → _Looking forward / Onward_
- **Concise**: none — keep short → _Best_

## Rules
- Subject ≤78 chars
- Personal note ≤200 chars
- Always sign off

## Done when
- Correct subject template used
- Subject ≤78 chars, personal note ≤200 chars
- All placeholders swapped
- Signed off
