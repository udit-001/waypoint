package skills

import _ "embed"

//go:embed job-tracker/SKILL.md
var SkillFile []byte

const SkillName = "job-tracker"

