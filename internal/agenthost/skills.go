package agenthost

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	assets "github.com/z-chenhao/ad-agent"
	ar "github.com/z-chenhao/ad-agent/internal/runtime"
)

type skillManifestEntry struct {
	Name          string   `json:"name"`
	Status        string   `json:"status"`
	Description   string   `json:"description"`
	OfficialAreas []string `json:"official_api_areas"`
	RequiredTools []string `json:"required_tools"`
}

type skillManifest struct {
	Version string               `json:"version"`
	Skills  []skillManifestEntry `json:"skills"`
}

type skillRegistry struct {
	active []skillManifestEntry
	byName map[string]string
}

func loadSkillRegistry() (skillRegistry, error) {
	b, err := assets.Assets.ReadFile("skills/manifest.json")
	if err != nil {
		return skillRegistry{}, err
	}
	var manifest skillManifest
	if err := json.Unmarshal(b, &manifest); err != nil {
		return skillRegistry{}, err
	}
	if manifest.Version != "1" {
		return skillRegistry{}, errors.New("unsupported_skill_manifest")
	}
	r := skillRegistry{byName: map[string]string{}}
	seen := map[string]bool{}
	for _, entry := range manifest.Skills {
		if entry.Name == "" || entry.Description == "" || seen[entry.Name] {
			return skillRegistry{}, fmt.Errorf("invalid skill manifest entry %q", entry.Name)
		}
		seen[entry.Name] = true
		if entry.Status != "active" && entry.Status != "staged" {
			return skillRegistry{}, fmt.Errorf("invalid skill status for %q", entry.Name)
		}
		path := "skills/" + entry.Name + "/SKILL.md"
		if entry.Status == "staged" {
			path = "skills/_staged/" + entry.Name + "/SKILL.md"
		}
		body, err := assets.Assets.ReadFile(path)
		if err != nil {
			return skillRegistry{}, fmt.Errorf("load %s: %w", entry.Name, err)
		}
		name, description, instructions, err := parseSkill(string(body))
		if err != nil || name != entry.Name || description != entry.Description {
			return skillRegistry{}, fmt.Errorf("skill metadata mismatch for %q", entry.Name)
		}
		if entry.Status == "active" {
			r.active = append(r.active, entry)
			r.byName[entry.Name] = instructions
		}
	}
	sort.Slice(r.active, func(i, j int) bool { return r.active[i].Name < r.active[j].Name })
	return r, nil
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
			name = strings.TrimSpace(value)
		case "description":
			description = strings.TrimSpace(value)
		}
	}
	if name == "" || description == "" {
		return "", "", "", errors.New("skill frontmatter requires name and description")
	}
	return name, description, strings.TrimSpace(parts[2]), nil
}

func (r skillRegistry) names() []string {
	names := make([]string, 0, len(r.active))
	for _, skill := range r.active {
		names = append(names, skill.Name)
	}
	return names
}

func (r skillRegistry) index() string {
	var b strings.Builder
	for _, skill := range r.active {
		fmt.Fprintf(&b, "- `%s` — %s\n", skill.Name, skill.Description)
	}
	return strings.TrimSpace(b.String())
}

func (r skillRegistry) get(name string) (string, bool) {
	body, ok := r.byName[name]
	return body, ok
}

func (r skillRegistry) validateTools(tools []ar.Tool) error {
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
