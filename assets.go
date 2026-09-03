package adagent

import "embed"

// Assets contains the static product contract and explicitly loaded operating recipes.
//
//go:embed AGENT.md skills/*/SKILL.md
var Assets embed.FS
