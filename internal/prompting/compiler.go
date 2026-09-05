// Package prompting assembles the stable system prefix and bounded per-turn context.
// It is private product composition, not a provider prompt-caching protocol.
package prompting

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"
)

const MaxSystemWords = 1024

type Scope string

const (
	Advertiser Scope = "advertiser"
	Manager    Scope = "manager"
)

type Options struct {
	Scope      Scope
	ScopeAsset string
	ToolNames  []string
	SkillIndex string
}

type Plan struct {
	System string
}

// Compile builds a byte-stable system prompt for one deployment capability set.
// Per-turn values must never be passed here.
func Compile(assets fs.FS, options Options) (Plan, error) {
	if options.Scope != Advertiser && options.Scope != Manager {
		return Plan{}, errors.New("invalid_prompt_scope")
	}
	if strings.TrimSpace(options.ScopeAsset) == "" {
		return Plan{}, errors.New("missing_scope_prompt")
	}
	kernel, err := fs.ReadFile(assets, "prompts/ad-agent-system.md")
	if err != nil {
		return Plan{}, err
	}
	scope, err := fs.ReadFile(assets, options.ScopeAsset)
	if err != nil {
		return Plan{}, err
	}
	names := append([]string(nil), options.ToolNames...)
	sort.Strings(names)
	capabilities := capabilitySummary(options.Scope, names)
	index := strings.TrimSpace(options.SkillIndex)
	if index == "" {
		index = "No operating guides are installed for this deployment."
	}
	system := strings.TrimSpace(string(kernel)) + "\n\n" + strings.TrimSpace(string(scope)) +
		"\n\n## Deployment capabilities\n\n" + capabilities +
		"\n\n## Installed operating guides\n\nLoad relevant guides selectively for specialist judgment.\n\n" + index
	if len(strings.Fields(system)) > MaxSystemWords {
		return Plan{}, errors.New("system_prompt_too_large")
	}
	return Plan{System: system}, nil
}

func capabilitySummary(scope Scope, names []string) string {
	tools := make(map[string]bool, len(names))
	for _, name := range names {
		tools[name] = true
	}
	available := func(names ...string) bool {
		for _, name := range names {
			if tools[name] {
				return true
			}
		}
		return false
	}
	var lines []string
	workspace := "Advertiser"
	if scope == Manager {
		workspace = "Manager"
	}
	lines = append(lines, "- Workspace: "+workspace+". It is fixed for this session.")
	if available("run_analysis", "run_manager_analysis") {
		lines = append(lines, "- Bounded read-only analysis is available; it cannot grant object provenance or mutation authority.")
	} else {
		lines = append(lines, "- Analysis delegation is unavailable; do not imply that a separate analysis run occurred.")
	}
	if available("stage_budget_change", "stage_status_change", "stage_account_budget_change", "stage_account_status_change") {
		lines = append(lines, "- Budget and delivery drafts are available. They remain unapplied until the host approval surface executes them.")
	} else {
		lines = append(lines, "- Budget and delivery drafts are unavailable; provide recommendations in text only.")
	}
	if available("stage_campaign_bundle", "stage_entity_create", "stage_account_entity_create") {
		lines = append(lines, "- Campaign/entity creation drafts are available only through the installed staging tools and their supported fields.")
	} else {
		lines = append(lines, "- Entity creation is unavailable in this deployment.")
	}
	if available("stage_ad_group_update", "stage_ad_creative_update", "stage_audience_create", "stage_automated_rule_create", "stage_comment_action", "stage_event_source_create") {
		lines = append(lines, "- Specialist operation drafts are available as declared by individual tool schemas; their presence does not imply other mutation capabilities.")
	}
	if available("present_metrics", "present_entities", "present_digest", "present_change_preview") {
		lines = append(lines, "- Structured presentation tools are available; use only the component types exposed by the tool list.")
	} else {
		lines = append(lines, "- Structured presentation tools are unavailable; return compact text without claiming a rendered component.")
	}
	if available("save_memory", "recall_memory", "delete_memory") {
		lines = append(lines, "- Account-scoped business memory tools are available under the kernel's memory restrictions.")
	} else {
		lines = append(lines, "- Interactive memory tools are unavailable in this scope.")
	}
	return strings.Join(lines, "\n")
}

type ContextBlock struct {
	Name  string
	JSON  []byte
	Limit int
}

type ContextOptions struct {
	Now             time.Time
	Timezone        string
	Blocks          []ContextBlock
	Notes           []string
	OperatorRequest string
}

// BuildContext keeps volatile values out of the stable system prefix. Each JSON block
// has an independent budget and is omitted whole rather than truncated into invalid data.
func BuildContext(options ContextOptions) (string, error) {
	if options.Now.IsZero() {
		options.Now = time.Now()
	}
	zone := strings.TrimSpace(options.Timezone)
	if zone == "" {
		zone = "UTC"
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		location = time.UTC
		zone = "UTC"
	}
	clock, _ := json.Marshal(struct {
		LocalTime string `json:"local_time"`
		Timezone  string `json:"timezone"`
	}{LocalTime: options.Now.In(location).Format(time.RFC3339), Timezone: zone})
	var out strings.Builder
	fmt.Fprintf(&out, "<runtime_context>%s</runtime_context>\n", clock)
	for _, block := range options.Blocks {
		if !validBlockName(block.Name) || block.Limit < 1 || !json.Valid(block.JSON) || bytes.ContainsAny(block.JSON, "<>") {
			return "", errors.New("invalid_context_block")
		}
		payload := block.JSON
		if len(payload) > block.Limit {
			payload = []byte(fmt.Sprintf(`{"omitted":true,"reason":"context_limit_exceeded","bytes":%d,"limit":%d}`, len(block.JSON), block.Limit))
		}
		fmt.Fprintf(&out, "<%s>%s</%s>\n", block.Name, payload, block.Name)
	}
	for _, note := range options.Notes {
		if note = strings.TrimSpace(note); note != "" {
			out.WriteString(note)
			out.WriteByte('\n')
		}
	}
	out.WriteString("Operator request:\n")
	out.WriteString(options.OperatorRequest)
	return out.String(), nil
}

func validBlockName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if (r < 'a' || r > 'z') && r != '_' {
			return false
		}
	}
	return true
}
