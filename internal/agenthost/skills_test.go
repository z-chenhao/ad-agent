package agenthost

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSkillManifestExposesOnlyActiveSkills(t *testing.T) {
	skills, err := loadSkillRegistry(true, true, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills.names()) != 5 {
		t.Fatalf("active skills = %d, want 5", len(skills.names()))
	}
	if _, ok := skills.get("measurement-and-automation"); !ok {
		t.Fatal("common-ads skill is not runtime-visible")
	}
	if _, ok := skills.get("account-operations"); !ok {
		t.Fatal("active skill is not loadable")
	}
	withoutCreator, err := loadSkillRegistry(false, true, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(withoutCreator.names()) != 5 {
		t.Fatalf("non-creator skills = %d, want 5", len(withoutCreator.names()))
	}
	if _, ok := skills.get("platform-native-campaigns"); ok {
		t.Fatal("staged platform-native workflow became operator-visible")
	}
	if _, ok := skills.get("creative-and-identity-operations"); ok {
		t.Fatal("overlapping creative inventory workflow became operator-visible")
	}
	registry, err := newRegistry(false, skills.names(), true, true, true, true)
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
		if !strings.Contains(names, "measurement-and-automation") || !strings.Contains(names, "account-operations") {
			t.Fatalf("unexpected generated skill enum: %s", names)
		}
		return
	}
	t.Fatal("load_skill tool not found")
}

func TestSkillManifestSeparatesAdvertiserAndManagerScopes(t *testing.T) {
	manager, err := LoadSkillRegistry("manager", true)
	if err != nil {
		t.Fatal(err)
	}
	if names := strings.Join(manager.Names(), ","); names != "manager-operations" {
		t.Fatalf("manager skill names = %q", names)
	}
	if _, ok := manager.Get("account-operations"); ok {
		t.Fatal("advertiser skill leaked into manager scope")
	}
	if _, ok := manager.Get("manager-operations"); !ok {
		t.Fatal("manager workflow is not loadable")
	}
	single, err := LoadSkillRegistry("advertiser", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := single.Get("manager-operations"); ok {
		t.Fatal("manager workflow leaked into advertiser scope")
	}
}
