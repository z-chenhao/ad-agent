package agenthost

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	assets "github.com/z-chenhao/ad-agent"
	ar "github.com/z-chenhao/ad-agent/internal/runtime"
)

type skillManifestEntry struct {
	Name          string   `json:"name"`
	Status        string   `json:"status"`
	Scopes        []string `json:"scopes"`
	Description   string   `json:"description"`
	OfficialAreas []string `json:"official_api_areas"`
	RequiredTools []string `json:"required_tools"`
}

type skillManifest struct {
	Version string               `json:"version"`
	Skills  []skillManifestEntry `json:"skills"`
}

type SkillRegistry struct {
	active   []skillManifestEntry
	byName   map[string]string
	reserved map[string]bool
}

func LoadSkillRegistry(scope string, capabilities ...bool) (SkillRegistry, error) {
	b, err := assets.Assets.ReadFile("skills/manifest.json")
	if err != nil {
		return SkillRegistry{}, err
	}
	var manifest skillManifest
	if err := json.Unmarshal(b, &manifest); err != nil {
		return SkillRegistry{}, err
	}
	if manifest.Version != "1" {
		return SkillRegistry{}, errors.New("unsupported_skill_manifest")
	}
	if scope != "advertiser" && scope != "manager" {
		return SkillRegistry{}, errors.New("invalid_skill_scope")
	}
	r := SkillRegistry{byName: map[string]string{}}
	canCreate := len(capabilities) > 0 && capabilities[0]
	commonAdsEnabled := len(capabilities) > 1 && capabilities[1]
	operationsEnabled := len(capabilities) > 2 && capabilities[2]
	operationsReaderEnabled := len(capabilities) > 3 && capabilities[3]
	seen := map[string]bool{}
	r.reserved = seen
	for _, entry := range manifest.Skills {
		if entry.Name == "" || entry.Description == "" || len(entry.Scopes) == 0 || seen[entry.Name] {
			return SkillRegistry{}, fmt.Errorf("invalid skill manifest entry %q", entry.Name)
		}
		scopeSeen := map[string]bool{}
		for _, entryScope := range entry.Scopes {
			if (entryScope != "advertiser" && entryScope != "manager") || scopeSeen[entryScope] {
				return SkillRegistry{}, fmt.Errorf("invalid skill scope for %q", entry.Name)
			}
			scopeSeen[entryScope] = true
		}
		seen[entry.Name] = true
		if entry.Status != "active" && entry.Status != "staged" {
			return SkillRegistry{}, fmt.Errorf("invalid skill status for %q", entry.Name)
		}
		path := "skills/" + entry.Name + "/SKILL.md"
		if entry.Status == "staged" {
			path = "skills/_staged/" + entry.Name + "/SKILL.md"
		}
		body, err := assets.Assets.ReadFile(path)
		if err != nil {
			return SkillRegistry{}, fmt.Errorf("load %s: %w", entry.Name, err)
		}
		name, description, instructions, err := parseSkill(string(body))
		if err != nil || name != entry.Name || description != entry.Description {
			return SkillRegistry{}, fmt.Errorf("skill metadata mismatch for %q", entry.Name)
		}
		if entry.Status == "active" && contains(entry.Scopes, scope) {
			if contains(entry.RequiredTools, "stage_entity_create") && !canCreate {
				continue
			}
			if requiresCommonAds(entry.RequiredTools) && !commonAdsEnabled {
				continue
			}
			if requiresOperations(entry.RequiredTools) && !operationsEnabled {
				continue
			}
			if requiresOperationsReader(entry.RequiredTools) && !operationsReaderEnabled {
				continue
			}
			r.active = append(r.active, entry)
			r.byName[entry.Name] = instructions
		}
	}
	sort.Slice(r.active, func(i, j int) bool { return r.active[i].Name < r.active[j].Name })
	return r, nil
}

func loadSkillRegistry(capabilities ...bool) (SkillRegistry, error) {
	canCreate := len(capabilities) > 0 && capabilities[0]
	commonAds := len(capabilities) > 1 && capabilities[1]
	operations := len(capabilities) > 2 && capabilities[2]
	operationsReader := len(capabilities) > 3 && capabilities[3]
	return LoadSkillRegistry("advertiser", canCreate, commonAds, operations, operationsReader)
}

func requiresOperations(tools []string) bool {
	for _, tool := range tools {
		if operationTools[tool] {
			return true
		}
	}
	return false
}
func requiresOperationsReader(tools []string) bool {
	for _, tool := range tools {
		if operationReaderTools[tool] {
			return true
		}
	}
	return false
}

func requiresCommonAds(tools []string) bool {
	for _, tool := range tools {
		if commonAdsTools[tool] {
			return true
		}
	}
	return false
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func parseSkill(text string) (name, description, body string, err error) {
	if !strings.HasPrefix(text, "---\n") {
		return "", "", "", errors.New("missing skill frontmatter")
	}
	parts := strings.SplitN(text, "---\n", 3)
	if len(parts) != 3 {
		return "", "", "", errors.New("malformed skill frontmatter")
	}
	for _, line := range strings.Split(parts[1], "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "name":
			name = parseFrontmatterScalar(value)
		case "description":
			description = parseFrontmatterScalar(value)
		}
	}
	if name == "" || description == "" {
		return "", "", "", errors.New("skill frontmatter requires name and description")
	}
	return name, description, strings.TrimSpace(parts[2]), nil
}

func parseFrontmatterScalar(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		if unquoted, err := strconv.Unquote(value); err == nil {
			return unquoted
		}
	}
	return value
}

func (r SkillRegistry) Names() []string {
	names := make([]string, 0, len(r.active))
	for _, skill := range r.active {
		names = append(names, skill.Name)
	}
	return names
}

func (r SkillRegistry) Index() string {
	var b strings.Builder
	for _, skill := range r.active {
		fmt.Fprintf(&b, "- `%s` — %s\n", skill.Name, skill.Description)
	}
	return strings.TrimSpace(b.String())
}

func (r SkillRegistry) Get(name string) (string, bool) {
	body, ok := r.byName[name]
	return body, ok
}

func (r SkillRegistry) ValidateTools(tools []ar.Tool) error {
	installed := make(map[string]bool, len(tools))
	for _, tool := range tools {
		installed[tool.Name] = true
	}
	for _, skill := range r.active {
		for _, name := range skill.RequiredTools {
			if !installed[name] {
				return fmt.Errorf("active skill %q requires missing tool %q", skill.Name, name)
			}
		}
	}
	return nil
}

func (r SkillRegistry) names() []string                     { return r.Names() }
func (r SkillRegistry) index() string                       { return r.Index() }
func (r SkillRegistry) get(name string) (string, bool)      { return r.Get(name) }
func (r SkillRegistry) validateTools(tools []ar.Tool) error { return r.ValidateTools(tools) }
