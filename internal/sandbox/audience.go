package sandbox

import "strings"

func (f *Backend) audienceMatches(id string, cohort Cohort) bool {
	definition, ok := f.operations.AudienceDefinitions[id]
	if !ok {
		return audienceMatchesCohort(id, cohort.ID)
	}
	if len(definition.LocationIDs) > 0 && !contains(definition.LocationIDs, cohort.Geo) {
		return false
	}
	if len(definition.Languages) > 0 && !contains(definition.Languages, cohort.Language) {
		return false
	}
	if len(definition.AgeGroups) > 0 && !contains(definition.AgeGroups, cohort.AgeGroup) {
		return false
	}
	if definition.Gender != "" && definition.Gender != "GENDER_UNLIMITED" && !strings.EqualFold(definition.Gender, cohort.Gender) {
		return false
	}
	if definition.Kind == "lookalike" {
		// Configurable breadth is approximated by cohort affinity; source members
		// are excluded. This is aggregate modeling, not platform audience matching.
		if audienceMatchesCohort(definition.SourceAudienceID, cohort.ID) {
			return false
		}
		return cohort.AudienceAffinity >= 1.3-float64(definition.LookalikeRatio)*.05
	}
	return true
}
