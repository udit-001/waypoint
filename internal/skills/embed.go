package skills

import "embed"

// Files embeds the waypoint skill directory (SKILL.md + references/*).
//go:embed waypoint
var Files embed.FS

const SkillName = "waypoint"
