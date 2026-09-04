package agenthost

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSkillManifestExposesOnlyActiveSkills(t *testing.T) {
	skills, err := loadSkillRegistry(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills.names()) != 10 {
		t.Fatalf("active skills = %d, want 10", len(skills.names()))
	}
	if _, ok := skills.get("measurement-and-attribution"); ok {
		t.Fatal("staged skill became runtime-visible")
	}
	if _, ok := skills.get("daily-account-briefing"); !ok {
		t.Fatal("active skill is not loadable")
	}
	withoutCreator, err := loadSkillRegistry(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(withoutCreator.names()) != 9 {
		t.Fatalf("non-creator skills = %d, want 9", len(withoutCreator.names()))
	}
	if _, ok := withoutCreator.get("sandbox-lifecycle"); ok {
		t.Fatal("creator-only skill became available without Creator")
	}
	registry, err := newRegistry(false, skills.names(), true)
	if err != nil {
		t.Fatal(err)
	}
	if err := skills.validateTools(registry.tools); err != nil {
		t.Fatal(err)
	}
	for _, tool := range registry.tools {
		if tool.Name != "load_skill" {
			continue
		}
		var schema struct {
			Properties map[string]struct {
				Enum []string `json:"enum"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(tool.Parameters, &schema); err != nil {
			t.Fatal(err)
		}
		names := strings.Join(schema.Properties["name"].Enum, ",")
		if strings.Contains(names, "measurement-and-attribution") || !strings.Contains(names, "daily-account-briefing") {
			t.Fatalf("unexpected generated skill enum: %s", names)
		}
		return
	}
	t.Fatal("load_skill tool not found")
}
