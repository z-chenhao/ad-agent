package adagent

import "embed"

// Assets contains the static product contract and the capability-gated operating recipes.
//
//go:embed prompts/ad-agent-system.md prompts/advertiser-scope.md prompts/manager-scope.md skills/manifest.json skills/*/SKILL.md skills/_staged/*/SKILL.md
var Assets embed.FS
