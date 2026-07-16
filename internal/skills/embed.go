package skills

import "embed"

// Files embeds the waypoint skill directory (SKILL.md + references/*).
//
//go:embed waypoint
var Files embed.FS

// All is the list of embedded skills installed by `waypoint skills install`.
var All = []string{"waypoint"}

const SkillName = "waypoint"
