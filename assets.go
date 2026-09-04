package adagent

import "embed"

// Assets contains the static product contract and the capability-gated operating recipes.
//
//go:embed prompts/ad-agent-system.md prompts/portfolio-agent-system.md skills/manifest.json skills/*/SKILL.md skills/_staged/*/SKILL.md
var Assets embed.FS
