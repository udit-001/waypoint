# Career Summary

Resume summary / professional bio. 5 styles.

## Options

- **style**: Standard | Impact | Technical | Executive | Entry-Level
- **emphasize**: Skills | Experience | Achievements | Balanced
- **length**: Brief (120) | Short (280) | Detailed (500 chars)
- **include contact**: bool

**Always** include target role from profile.

## Structures

- **Standard**: title · years-exp · core-skills · achievements · goal
- **Impact**: hook · key-result · value-prop · closing
- **Technical**: name-title · competencies · tools · exp-highlights · value-prop
- **Executive**: leadership-brand · strategic-impact · team-scale · vision
- **Entry-Level**: education · relevant-skills · internships/projects · motivation · potential

## Steps

1. `waypoint profile show --json` — pull skills, experience, education
2. Pick style + emphasis from user request
3. Draft following the structure above; stay within length cap
4. If user wants comparison → generate one per style

**Done when**: summary fits the structure, stays within length cap, includes target role.
