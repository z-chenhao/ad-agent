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
	if !child && (len(canCreate) == 0 || !canCreate[0]) {
		filtered := r.tools[:0]
		for _, tool := range r.tools {
			if tool.Name != "stage_entity_create" {
				filtered = append(filtered, tool)
			}
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
