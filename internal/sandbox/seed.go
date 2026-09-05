package sandbox

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/z-chenhao/ad-agent/internal/ads"
)

var historicalSeedCache sync.Map

type historicalSeedSnapshot struct {
	Rows  []ads.Row            `json:"rows"`
	Model SimulationModelState `json:"model"`
}

// deliveryProfiles is the canonical seeded-ad inventory. Calibration lives in
// SimulationConfig.Ads; no high-level metric is sampled independently.
var deliveryProfiles = map[string]struct{}{
	"ad_prospect_creator": {}, "ad_prospect_demo": {}, "ad_interest_room": {},
	"ad_interest_before": {}, "ad_lal_unboxing": {}, "ad_lal_review": {},
	"ad_viewers_founder": {}, "ad_viewers_offer": {}, "ad_cart_proof": {},
	"ad_cart_urgency": {}, "ad_launch_teaser": {}, "ad_launch_collection": {},
}

// historicalSeed replays the same causal generator used by virtual-time advances.
// Facts are aggregated to ad/day rows only because the initial history is immutable
// product seed data; the evolved hidden state is retained for future delivery.
func (f *Backend) historicalSeed(days int) ([]ads.Row, error) {
	cacheInput, _ := json.Marshal([]any{f.environment, f.account.ID, f.account.LatestDate, days, f.model.Config})
	cacheHash := sha256.Sum256(cacheInput)
	cacheKey := hex.EncodeToString(cacheHash[:])
	if encoded, ok := historicalSeedCache.Load(cacheKey); ok {
		var snapshot historicalSeedSnapshot
		if json.Unmarshal(encoded.([]byte), &snapshot) == nil {
			f.model = snapshot.Model
			return snapshot.Rows, nil
		}
	}
	latest, _ := time.ParseInLocation(time.DateOnly, f.account.LatestDate, f.location)
	start := latest.AddDate(0, 0, -(days - 1))
	adIDs := make([]string, 0, len(deliveryProfiles))
	for id, entity := range f.entities {
		if entity.Level != ads.Ad {
			continue
		}
		if _, ok := deliveryProfiles[id]; !ok {
			// Created ads are valid after the seed window; only the canonical seed is
			// required to have explicit calibration.
			continue
		}
		if _, ok := f.model.Config.Ads[id]; !ok {
			return nil, errors.New("virtual account ad is missing causal calibration")
		}
		adIDs = append(adIDs, id)
	}
	if len(adIDs) != len(deliveryProfiles) {
		return nil, errors.New("virtual account causal calibration references an unknown ad")
	}
	sort.Strings(adIDs)
	rows := make([]ads.Row, 0, days*len(adIDs))
	for offset := 0; offset < days; offset++ {
		day := start.AddDate(0, 0, offset)
		daily := make(map[string]ads.Metrics, len(adIDs))
		for hour := 0; hour < 24; hour++ {
			instant := day.Add(time.Duration(hour) * time.Hour).UTC()
			adsAtHour := make([]ads.Entity, 0, len(adIDs))
			for _, adID := range adIDs {
				adsAtHour = append(adsAtHour, f.entities[adID])
			}
			for _, fact := range f.generateCausalHoursJoint(adsAtHour, instant) {
				daily[fact.AdID] = addHistoricalMetrics(daily[fact.AdID], fact.Metrics)
			}
		}
		for _, adID := range adIDs {
			rows = append(rows, ads.Row{EntityID: adID, Date: day.Format(time.DateOnly), Metrics: daily[adID]})
		}
	}
	if encoded, err := json.Marshal(historicalSeedSnapshot{Rows: rows, Model: f.model}); err == nil {
		historicalSeedCache.Store(cacheKey, encoded)
	}
	return rows, nil
}

func addHistoricalMetrics(left, right ads.Metrics) ads.Metrics {
	if left.Conversions == nil || left.Revenue == nil {
		left = ads.ZeroMetrics()
	}
	leftReach, rightReach := optionalInt(left.Reach), optionalInt(right.Reach)
	result := left.Add(right)
	reach := leftReach + rightReach
	result.Reach = &reach
	return result
}

func optionalInt(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
