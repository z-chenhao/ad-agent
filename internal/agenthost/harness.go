package agenthost

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/z-chenhao/ad-agent/internal/ads"
	ar "github.com/z-chenhao/ad-agent/internal/runtime"
	"github.com/z-chenhao/ad-agent/internal/store"
)

// harnessPolicy is repository-private orchestration policy. It projects one installed
// tool surface into grounding, mutation follow-through, and presentation semantics so
// every runtime adapter receives the same advertising behavior.
type harnessPolicy struct {
	tools                map[string]bool
	closingPresentations map[string]bool
}

type PublicCapability struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tools       []string `json:"tools"`
}

type PublicHarness struct {
	Capabilities           []PublicCapability `json:"capabilities"`
	Grounding              bool               `json:"grounding"`
	StagingFollowThrough   bool               `json:"staging_follow_through"`
	CloseOnPresentation    bool               `json:"close_on_presentation"`
	ReadConcurrency        bool               `json:"read_concurrency"`
	PartialPresentation    bool               `json:"partial_presentation"`
	AutomaticMemoryCapture bool               `json:"automatic_memory_capture"`
}

func newHarnessPolicy(reg registry) harnessPolicy {
	p := harnessPolicy{
		tools:                make(map[string]bool, len(reg.tools)),
		closingPresentations: map[string]bool{"present_suggestions": true},
	}
	for _, tool := range reg.tools {
		p.tools[tool.Name] = true
	}
	return p
}

func (h *Host) PublicHarness() PublicHarness {
	capabilities := make([]PublicCapability, 0, len(h.skills.active))
	for _, skill := range h.skills.active {
		capabilities = append(capabilities, PublicCapability{
			Name: skill.Name, Description: skill.Description,
			Tools: append([]string(nil), skill.RequiredTools...),
		})
	}
	return PublicHarness{
		Capabilities: capabilities, Grounding: true, StagingFollowThrough: true,
		CloseOnPresentation: true, ReadConcurrency: true, PartialPresentation: true,
		AutomaticMemoryCapture: h.AutomaticMemoryCapture,
	}
}

type groundingRead struct {
	Name      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
	Result    ar.ToolResult   `json:"result"`
}

func (p harnessPolicy) groundingCall(message string, account ads.Account) *ar.Call {
	lower := strings.ToLower(message)
	if p.tools["get_pending_changes"] && containsAny(lower,
		"approve", "apply", "pending change", "draft", "discard", "reconcile",
	) {
		return &ar.Call{ID: store.ID("ground"), Name: "get_pending_changes", Arguments: json.RawMessage(`{}`), Round: 0}
	}
	if p.tools["get_performance_report"] && containsAny(lower,
		"performance", "spend", "roas", "cpa", "cpc", "cpm", "ctr", "cvr",
		"conversion", "impression", "click", "revenue", "purchase value", "pacing",
		"diagnose", "compare", "trend", "daily briefing",
	) {
		end, err := time.Parse(time.DateOnly, account.LatestDate)
		if err != nil {
			return nil
		}
		q := ads.ReportQuery{Level: ads.Campaign, Start: end.AddDate(0, 0, -6).Format(time.DateOnly), End: account.LatestDate}
		arguments, _ := json.Marshal(q)
		return &ar.Call{ID: store.ID("ground"), Name: "get_performance_report", Arguments: arguments, Round: 0}
	}
	return nil
}

func containsAny(text string, markers ...string) bool {
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func changeRequested(message string) bool {
	lower := strings.ToLower(message)
	action := containsAny(lower,
		"change", "update", "set ", "increase", "decrease", "raise", "lower",
		"enable", "disable", "pause", "resume", "create", "launch", "edit",
		"adjust", "draft", "stage",
	)
	subject := containsAny(lower,
		"budget", "status", "campaign", "ad group", "adgroup", " ad ", "rule",
		"audience", "creative", "comment", "bid", "schedule", "targeting",
	)
	return action && subject
}

const stagingFollowThroughReminder = `The operator requested an advertising change, but this turn ended without a stage tool attempt. If the target and new value are sufficiently grounded, call the matching stage tool now and present its exact preview. If a required typed capability or fact is unavailable, state that concrete block. Do not merely repeat a recommendation and do not claim anything was applied.`

func addUsage(a, b ar.Usage) ar.Usage {
	return ar.Usage{
		Input: a.Input + b.Input, Output: a.Output + b.Output,
		CacheRead: a.CacheRead + b.CacheRead, CacheWrite: a.CacheWrite + b.CacheWrite,
	}
}
