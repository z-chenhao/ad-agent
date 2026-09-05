package sandbox

import (
	"math"
	"strings"

	"github.com/z-chenhao/ad-agent/internal/ads"
)

// reportReach estimates unique reach within the selected window from aggregate
// cohort occupancy. Historical incremental reach never substitutes for window reach.
// Uniform occupancy is an explicit modeling assumption, not person-level tracking.
func (f *Backend) reportReach(level ads.Level, id, start, end string) *int64 {
	counts := map[string]int64{}
	for key, cohorts := range f.model.ReportExposure {
		index := strings.LastIndex(key, "/")
		if index < 0 {
			continue
		}
		adID, day := key[:index], key[index+1:]
		if day < start || day > end {
			continue
		}
		entity, ok := f.entities[adID]
		if !ok {
			continue
		}
		for entity.Level != level && level != ads.Advertiser {
			var exists bool
			entity, exists = f.entities[entity.ParentID]
			if !exists {
				break
			}
		}
		if level != ads.Advertiser && id != "" && entity.ID != id {
			continue
		}
		for cohort, n := range cohorts {
			counts[cohort] += n
		}
	}
	if len(counts) == 0 {
		return nil
	}
	var reach int64
	for _, cohort := range f.model.Config.Cohorts {
		n := counts[cohort.ID]
		population := float64(cohort.ReachableUsers)
		reach += int64(math.Round(math.Min(float64(n), population*(-math.Expm1(-float64(n)/population)))))
	}
	return &reach
}
