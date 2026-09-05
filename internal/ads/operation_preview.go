package ads

import (
	"encoding/json"
	"sort"
	"strings"
)

// CreativeReviewBefore uses the same field names as the requested patch.
func CreativeReviewBefore(detail AdDetail) OperationRequest {
	v := AdCreativeUpdateSpec{AdID: detail.Ad.ID, PrimaryText: detail.PrimaryText, CallToAction: detail.CallToAction, DestinationURL: detail.DestinationURL}
	if detail.Creative != nil {
		v.AssetID = detail.Creative.ID
		v.AssetKind = detail.Creative.Kind
	}
	if detail.Identity != nil {
		v.IdentityID = detail.Identity.ID
		v.IdentityType = detail.Identity.Kind
	}
	return OperationRequest{Kind: UpdateAdCreative, AdUpdate: &v}
}

// OperationReviewLines projects the exact typed request into a deterministic review.
// Arrays stay intact, so exclusions and placements cannot disappear into a summary.
func OperationReviewLines(request OperationRequest, before any) []ChangeLine {
	encode := func(value any) map[string]any {
		data, _ := json.Marshal(value)
		result := map[string]any{}
		_ = json.Unmarshal(data, &result)
		return result
	}
	root := encode(request)
	delete(root, "kind")
	previous := encode(before)
	lines := []ChangeLine{}
	var visit func(map[string]any, map[string]any, string)
	visit = func(values, old map[string]any, prefix string) {
		keys := make([]string, 0, len(values))
		for key := range values {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			path := strings.TrimPrefix(prefix+"."+key, ".")
			if list, ok := values[key].([]any); ok && len(list) > 0 {
				if _, objects := list[0].(map[string]any); objects {
					for i, item := range list {
						encoded, _ := json.Marshal(i + 1)
						visit(item.(map[string]any), nil, path+"["+string(encoded)+"]")
					}
					continue
				}
			}
			if object, ok := values[key].(map[string]any); ok {
				prior, _ := old[key].(map[string]any)
				visit(object, prior, path)
				continue
			}
			text := func(v any) string {
				if v == nil {
					return ""
				}
				if s, ok := v.(string); ok {
					return s
				}
				b, _ := json.Marshal(v)
				return string(b)
			}
			lines = append(lines, ChangeLine{Resource: string(request.Kind), Field: path, Before: text(old[key]), After: text(values[key])})
		}
	}
	visit(root, previous, "")
	return lines
}
