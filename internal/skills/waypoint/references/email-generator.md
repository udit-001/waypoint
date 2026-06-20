# Email generator

Drafts job emails from job + profile data.

## Types
`application` · `followUp` · `thankYou` · `networking` · `referralRequest` · `offerAcceptance` · `rejectionResponse`

## Options
- tone: Formal|Casual|Creative|Concise (Formal)
- include salary: bool (no)
- focus: Skills|Experience|Education|Mixed (Mixed)
- personal note/hook: bool (yes)

## Steps
1. `waypoint get <id>` → pull job
2. read profile (name, skills, exp)
3. pick type from request
4. draft from template below
5. rules: subject ≤78 chars · personal note ≤200 chars · always sign off

## Templates
| type | subject |
|------|---------|
| application | `Application for {{position}} at {{company}}` |
| followUp | `Follow-Up: {{position}} Application` |
| thankYou | `Thank You - {{position}} Interview` |
| networking | `Connecting: {{position}} Interest at {{company}}` |
| referralRequest | `Referral Request: {{position}} at {{company}}` |
| offerAcceptance | `Offer Acceptance: {{position}} at {{company}}` |
| rejectionResponse | `Thank You — {{position}} at {{company}}` |

Tone adjectives: Formal (proven, established, seasoned) · Casual (passionate, enthusiastic, curious) · Creative (innovative, bold, dynamic) · Concise (none, keep short).

Closings: Best regards / Sincerely / Cheers / Yours faithfully.

Swap `{{company}}` `{{position}}` `{{contactName}}` w/ real values.
