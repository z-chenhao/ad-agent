package ads

import (
	"strings"
	"testing"
)

func TestOperationReviewIncludesExactTargetingAndCreativeBefore(t *testing.T) {
	request := OperationRequest{Kind: UpdateAdGroup, AdGroupUpdate: &AdGroupUpdateSpec{AdGroupID: "g1", LocationIDs: []string{"US", "CA"}, ExcludedAudienceIDs: []string{"purchasers"}}}
	lines := OperationReviewLines(request, OperationRequest{Kind: UpdateAdGroup, AdGroupUpdate: &AdGroupUpdateSpec{AdGroupID: "g1", LocationIDs: []string{"GB"}}})
	location, exclusion := false, false
	for _, line := range lines {
		if strings.HasSuffix(line.Field, "location_ids") {
			location = line.Before == `["GB"]` && line.After == `["US","CA"]`
		}
		if strings.HasSuffix(line.Field, "excluded_audience_ids") {
			exclusion = line.After == `["purchasers"]`
		}
	}
	if !location || !exclusion {
		t.Fatalf("incomplete preview: %+v", lines)
	}
	before := CreativeReviewBefore(AdDetail{Ad: Entity{ID: "ad1"}, DestinationURL: "https://example.com/old"})
	request = OperationRequest{Kind: UpdateAdCreative, AdUpdate: &AdCreativeUpdateSpec{AdID: "ad1", DestinationURL: "https://example.com/new"}}
	lines = OperationReviewLines(request, before)
	for _, line := range lines {
		if strings.HasSuffix(line.Field, "destination_url") && line.Before != "https://example.com/old" {
			t.Fatalf("missing creative before: %+v", lines)
		}
	}
}
