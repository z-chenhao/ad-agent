package agenthost

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"github.com/santhosh-tekuri/jsonschema/v6"
	ar "github.com/z-chenhao/ad-agent/internal/runtime"
)

//go:embed tools.json analysis-tools.json
var toolFiles embed.FS

type registry struct {
	tools   []ar.Tool
	schemas map[string]*jsonschema.Schema
}

func newRegistry(child bool, skillNames []string, canCreate ...bool) (registry, error) {
	name := "tools.json"
	if child {
		name = "analysis-tools.json"
	}
	b, err := toolFiles.ReadFile(name)
	if err != nil {
		return registry{}, err
	}
	r := registry{schemas: map[string]*jsonschema.Schema{}}
	if err = json.Unmarshal(b, &r.tools); err != nil {
		return r, err
	}
	if !child {
		creatorEnabled := len(canCreate) > 0 && canCreate[0]
		commonAdsEnabled := len(canCreate) > 1 && canCreate[1]
		operationsEnabled := len(canCreate) > 2 && canCreate[2]
		operationsReaderEnabled := len(canCreate) > 3 && canCreate[3]
		filtered := r.tools[:0]
		for _, tool := range r.tools {
			if tool.Name == "stage_entity_create" && !creatorEnabled {
				continue
			}
			if commonAdsTools[tool.Name] && !commonAdsEnabled {
				continue
			}
			if operationTools[tool.Name] && !operationsEnabled {
				continue
			}
			if operationReaderTools[tool.Name] && !operationsReaderEnabled {
				continue
			}
			filtered = append(filtered, tool)
		}
		r.tools = filtered
	}
	if !child {
		for i := range r.tools {
			if r.tools[i].Name != "load_skill" {
				continue
			}
			var schema map[string]any
			if err := json.Unmarshal(r.tools[i].Parameters, &schema); err != nil {
				return r, err
			}
			properties := schema["properties"].(map[string]any)
			name := properties["name"].(map[string]any)
			name["enum"] = skillNames
			r.tools[i].Parameters, err = json.Marshal(schema)
			if err != nil {
				return r, err
			}
		}
	}
	for _, t := range r.tools {
		var v any
		dec := json.NewDecoder(bytes.NewReader(t.Parameters))
		dec.UseNumber()
		if err = dec.Decode(&v); err != nil {
			return r, err
		}
		c := jsonschema.NewCompiler()
		uri := "https://ad-agent.invalid/" + t.Name
		if err = c.AddResource(uri, v); err != nil {
			return r, err
		}
		s, err := c.Compile(uri)
		if err != nil {
			return r, err
		}
		r.schemas[t.Name] = s
	}
	return r, nil
}

var operationTools = map[string]bool{
	"stage_campaign_bundle": true, "stage_ad_group_update": true,
	"stage_ad_creative_update": true, "stage_audience_create": true,
	"stage_automated_rule_create": true, "stage_comment_action": true,
	"stage_event_source_create": true,
}

var operationReaderTools = map[string]bool{
	"list_comments": true, "get_billing_balance": true, "list_billing_transactions": true,
}

var commonAdsTools = map[string]bool{
	"list_identities": true, "list_creative_assets": true, "get_creative_review": true,
	"list_audiences": true, "get_audience": true, "get_audience_overlap": true,
	"get_targeting_options": true, "list_event_sources": true, "get_event_stats": true,
	"get_optimization_events": true, "get_attribution_settings": true,
	"list_lead_forms": true, "get_lead_form": true, "list_catalogs": true,
	"get_catalog_feed_health": true, "get_catalog_product_health": true,
	"list_automated_rules": true, "get_automated_rule_results": true,
}

func (r registry) validate(c ar.Call) error {
	s, ok := r.schemas[c.Name]
	if !ok {
		return errors.New("unknown_tool")
	}
	if len(c.Arguments) > 16384 {
		return errors.New("arguments_too_large")
	}
	var v any
	dec := json.NewDecoder(bytes.NewReader(c.Arguments))
	dec.UseNumber()
	if dec.Decode(&v) != nil {
		return errors.New("invalid_json")
	}
	if s.Validate(v) != nil {
		return errors.New("invalid_arguments")
	}
	return nil
}
func decode[T any](b json.RawMessage) (T, error) {
	var v T
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	err := d.Decode(&v)
	return v, err
}
