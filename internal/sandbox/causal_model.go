package sandbox

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/z-chenhao/ad-agent/internal/ads"
)

const causalModelVersion = "causal-auction-v1"

// AssumptionClass keeps facts, replaceable formulas, and tunable values distinct.
// None of the ModelingAssumption entries claim a TikTok or Meta implementation.
type AssumptionClass string

const (
	PlatformInvariant    AssumptionClass = "PlatformInvariant"
	ModelingAssumption   AssumptionClass = "ModelingAssumption"
	CalibrationParameter AssumptionClass = "CalibrationParameter"
)

type ModelStatement struct {
	Name        string          `json:"name"`
	Class       AssumptionClass `json:"class"`
	Description string          `json:"description"`
}

// ModelStatements is developer documentation, not an Agent capability.
func ModelStatements() []ModelStatement {
	return []ModelStatement{
		{Name: "derived_metrics", Class: PlatformInvariant, Description: "CTR, CVR, CPM, CPC, CPA, and ROAS are derived from event counts, spend, and revenue."},
		{Name: "budget_cap", Class: PlatformInvariant, Description: "Cumulative spend cannot exceed the applicable budget."},
		{Name: "auction_score", Class: ModelingAssumption, Description: "A generic multiplicative value ranks eligible participants; it is not a reverse-engineered platform formula."},
		{Name: "concave_bid_utility", Class: ModelingAssumption, Description: "log(1+bid) gives bid diminishing returns."},
		{Name: "second_score_clearing", Class: ModelingAssumption, Description: "The next-best score approximates a generic clearing price."},
		{Name: "joint_account_auction", Class: ModelingAssumption, Description: "One ad is selected per participating ad group before overlapping ad groups and external bidders are ranked together."},
		{Name: "opportunity_aware_pacing", Class: ModelingAssumption, Description: "Remaining budget and forecast eligible opportunities determine a smooth participation probability."},
		{Name: "fatigue_and_saturation", Class: ModelingAssumption, Description: "Separate exponential frequency decays approximate creative fatigue and audience saturation."},
		{Name: "calibration", Class: CalibrationParameter, Description: "Market, action, pacing, fatigue, learning, attribution, and reporting coefficients are configurable."},
	}
}

type Cohort struct {
	AgeGroup          string  `json:"age_group,omitempty"`
	Gender            string  `json:"gender,omitempty"`
	ID                string  `json:"id"`
	Geo               string  `json:"geo"`
	Language          string  `json:"language"`
	Placement         string  `json:"placement"`
	Population        int64   `json:"population"`
	DailyActiveRate   float64 `json:"daily_active_rate"`
	AudienceAffinity  float64 `json:"audience_affinity"`
	PurchaseIntent    float64 `json:"purchase_intent"`
	BaseCTR           float64 `json:"base_ctr"`
	BaseCVR           float64 `json:"base_cvr"`
	MarketCompetition float64 `json:"market_competition"`
	ReachableUsers    int64   `json:"reachable_users"`
}

type MarketEvent struct {
	Name                      string    `json:"name"`
	Start                     time.Time `json:"start"`
	End                       time.Time `json:"end"`
	TrafficMultiplier         float64   `json:"traffic_multiplier"`
	PurchaseIntentMultiplier  float64   `json:"purchase_intent_multiplier"`
	CompetitorCountMultiplier float64   `json:"competitor_count_multiplier"`
	CompetitorBidMultiplier   float64   `json:"competitor_bid_multiplier"`
}

// SimulationConfig is experimental and Sandbox-private. It is deliberately not part
// of AdBackend, the Agent tool surface, or a promise about a real ad platform.
type SimulationConfig struct {
	Seed                         int64                    `json:"seed"`
	Cohorts                      []Cohort                 `json:"cohorts"`
	Ads                          map[string]AdCalibration `json:"ads"`
	EligiblePlacements           []string                 `json:"eligible_placements"`
	MarketEvents                 []MarketEvent            `json:"market_events,omitempty"`
	MaxParticipation             float64                  `json:"max_participation"`
	PacingSmoothness             float64                  `json:"pacing_smoothness"`
	ExpectedWinRate              float64                  `json:"expected_win_rate"`
	InternalSelectionTemperature float64                  `json:"internal_selection_temperature"`
	BidUtilityScale              float64                  `json:"bid_utility_scale"`
	CompetitorCountMean          float64                  `json:"competitor_count_mean"`
	CompetitorBidMean            float64                  `json:"competitor_bid_mean"`
	CompetitorBidSigma           float64                  `json:"competitor_bid_sigma"`
	CompetitorAuctionValueMean   float64                  `json:"competitor_auction_value_mean"`
	CompetitorQualityMean        float64                  `json:"competitor_quality_mean"`
	ClickObjectiveValueScale     float64                  `json:"click_objective_value_scale"`
	AuctionSamples               int                      `json:"auction_samples"`
	FloorImpressionCost          float64                  `json:"floor_impression_cost"`
	BidToImpressionCost          float64                  `json:"bid_to_impression_cost"`
	SaturationThreshold          float64                  `json:"saturation_threshold"`
	SaturationLambda             float64                  `json:"saturation_lambda"`
	FatigueThreshold             float64                  `json:"fatigue_threshold"`
	FatigueLambda                float64                  `json:"fatigue_lambda"`
	LearningClickThreshold       float64                  `json:"learning_click_threshold"`
	LearningConversionThreshold  float64                  `json:"learning_conversion_threshold"`
	LearningMajorChangeRatio     float64                  `json:"learning_major_change_ratio"`
	ExplorationStrength          float64                  `json:"exploration_strength"`
	TrackingCoverage             float64                  `json:"tracking_coverage"`
	AttributionDelayHours        int                      `json:"attribution_delay_hours"`
	ReportingLatencyHours        int                      `json:"reporting_latency_hours"`
	ViewThroughRate              float64                  `json:"view_through_rate"`
	ProductQuality               float64                  `json:"product_quality"`
	PriceAttractiveness          float64                  `json:"price_attractiveness"`
	LandingPageQuality           float64                  `json:"landing_page_quality"`
	Availability                 float64                  `json:"availability"`
	AOVLogMean                   float64                  `json:"aov_log_mean"`
	AOVLogSigma                  float64                  `json:"aov_log_sigma"`
	Debug                        bool                     `json:"debug"`
}

func DefaultSimulationConfig() SimulationConfig {
	return SimulationConfig{
		Seed:               7823471,
		Ads:                defaultAdCalibrations(),
		EligiblePlacements: []string{"FEED"},
		Cohorts: []Cohort{
			{ID: "broad_home", Geo: "US", Language: "en", Placement: "FEED", Population: 420000, DailyActiveRate: .58, AudienceAffinity: 1.00, PurchaseIntent: .82, BaseCTR: .013, BaseCVR: .012, MarketCompetition: 1.00, ReachableUsers: 420000},
			{ID: "engaged_viewers", Geo: "US", Language: "en", Placement: "FEED", Population: 92000, DailyActiveRate: .66, AudienceAffinity: 1.24, PurchaseIntent: 1.32, BaseCTR: .017, BaseCVR: .016, MarketCompetition: 1.12, ReachableUsers: 92000},
			{ID: "high_intent_cart", Geo: "US", Language: "en", Placement: "FEED", Population: 18500, DailyActiveRate: .51, AudienceAffinity: 1.46, PurchaseIntent: 1.85, BaseCTR: .021, BaseCVR: .024, MarketCompetition: 1.20, ReachableUsers: 18500},
		},
		MaxParticipation: .72, PacingSmoothness: 1.8, ExpectedWinRate: .24,
		InternalSelectionTemperature: .35,
		BidUtilityScale:              1, CompetitorCountMean: 4.5, CompetitorBidMean: 1.45,
		CompetitorBidSigma: .38, CompetitorAuctionValueMean: .060,
		CompetitorQualityMean: 1, ClickObjectiveValueScale: 3.5,
		AuctionSamples: 64, FloorImpressionCost: .003,
		BidToImpressionCost: .0048, SaturationThreshold: 2.2, SaturationLambda: .16,
		FatigueThreshold: 2.8, FatigueLambda: .20, LearningClickThreshold: 900,
		LearningConversionThreshold: 32, LearningMajorChangeRatio: .20,
		ExplorationStrength: .22, TrackingCoverage: .90,
		AttributionDelayHours: 2, ReportingLatencyHours: 1, ViewThroughRate: .000018,
		ProductQuality: 1.06, PriceAttractiveness: .96, LandingPageQuality: 1.02,
		Availability: .98, AOVLogMean: math.Log(78), AOVLogSigma: .32,
	}
}

func (c SimulationConfig) validate() error {
	if len(c.Cohorts) == 0 || len(c.Ads) == 0 || len(c.EligiblePlacements) == 0 || c.Seed == 0 || c.MaxParticipation <= 0 || c.MaxParticipation > 1 ||
		c.PacingSmoothness <= 0 || c.ExpectedWinRate <= 0 || c.ExpectedWinRate > 1 || c.InternalSelectionTemperature <= 0 || c.CompetitorCountMean <= 0 ||
		c.CompetitorBidMean <= 0 || c.CompetitorBidSigma < 0 || c.CompetitorAuctionValueMean <= 0 ||
		c.CompetitorQualityMean <= 0 || c.ClickObjectiveValueScale <= 0 || c.AuctionSamples < 1 || c.AuctionSamples > 512 ||
		c.FloorImpressionCost <= 0 || c.BidToImpressionCost <= 0 || c.TrackingCoverage < 0 || c.TrackingCoverage > 1 ||
		c.SaturationThreshold < 0 || c.SaturationLambda < 0 || c.FatigueThreshold < 0 || c.FatigueLambda < 0 ||
		c.LearningClickThreshold <= 0 || c.LearningConversionThreshold <= 0 || c.LearningMajorChangeRatio <= 0 || c.ExplorationStrength < 0 ||
		c.AttributionDelayHours < 0 || c.ReportingLatencyHours < 0 || c.ViewThroughRate < 0 || c.ProductQuality <= 0 ||
		c.PriceAttractiveness <= 0 || c.LandingPageQuality <= 0 || c.Availability < 0 || c.Availability > 1 {
		return errors.New("invalid sandbox simulation calibration")
	}
	seen := map[string]bool{}
	for _, v := range c.Cohorts {
		if v.ID == "" || seen[v.ID] || v.Geo == "" || v.Language == "" || v.Placement == "" || v.Population < 1 || v.ReachableUsers < 1 || v.ReachableUsers > v.Population ||
			v.DailyActiveRate <= 0 || v.DailyActiveRate > 1 || v.AudienceAffinity <= 0 || v.PurchaseIntent <= 0 ||
			v.BaseCTR <= 0 || v.BaseCTR >= 1 || v.BaseCVR <= 0 || v.BaseCVR >= 1 || v.MarketCompetition <= 0 {
			return errors.New("invalid sandbox cohort calibration")
		}
		seen[v.ID] = true
	}
	for id, v := range c.Ads {
		if id == "" || v.DefaultBid <= 0 || v.CreativeQuality <= 0 || v.Novelty <= 0 || v.AssetID == "" || v.ReviewState == "" {
			return errors.New("invalid sandbox ad calibration")
		}
	}
	for _, event := range c.MarketEvents {
		if event.Name == "" || event.End.Before(event.Start) || event.TrafficMultiplier <= 0 || event.PurchaseIntentMultiplier <= 0 || event.CompetitorCountMultiplier <= 0 || event.CompetitorBidMultiplier <= 0 {
			return errors.New("invalid sandbox market event")
		}
	}
	return nil
}

type CohortDeliveryState struct {
	CumulativeImpressions int64   `json:"cumulative_impressions"`
	UniqueReachedUsers    float64 `json:"unique_reached_users"`
}

type CreativeCohortDeliveryState struct {
	CumulativeImpressions int64   `json:"cumulative_impressions"`
	UniqueReachedUsers    float64 `json:"unique_reached_users"`
}

type CreativeDeliveryState struct {
	AssetID               string `json:"asset_id"`
	FirstSeenHour         int64  `json:"first_seen_hour"`
	CumulativeImpressions int64  `json:"cumulative_impressions"`
}

type LearningState struct {
	Fingerprint string  `json:"fingerprint"`
	Budget      float64 `json:"budget"`
	Bid         float64 `json:"bid"`
	Clicks      int64   `json:"clicks"`
	Conversions float64 `json:"conversions"`
	Progress    float64 `json:"progress"`
}

type SimulationModelState struct {
	LifetimeSpend   map[string]float64                     `json:"lifetime_spend"`
	ReportExposure  map[string]map[string]int64            `json:"report_exposure"`
	Version         string                                 `json:"version"`
	Config          SimulationConfig                       `json:"config"`
	AudienceCohorts map[string]CohortDeliveryState         `json:"audience_cohort_state"`
	CreativeCohorts map[string]CreativeCohortDeliveryState `json:"creative_cohort_state"`
	Creatives       map[string]CreativeDeliveryState       `json:"creative_state"`
	Learning        map[string]LearningState               `json:"learning_state"`
	DailySpend      map[string]float64                     `json:"daily_spend"`
	GeneratedSteps  int64                                  `json:"generated_steps"`
}

type AttributionBreakdown struct {
	ClickThrough int64 `json:"click_through"`
	ViewThrough  int64 `json:"view_through"`
}

type CausalTrace struct {
	Hour                           time.Time `json:"hour"`
	AdID                           string    `json:"ad_id"`
	Opportunities                  int64     `json:"opportunities"`
	EligibleOpportunities          int64     `json:"eligible_opportunities"`
	AuctionParticipations          int64     `json:"auction_participations"`
	AverageCompetitorScore         float64   `json:"average_competitor_score"`
	AuctionWins                    int64     `json:"auction_wins"`
	WinRate                        float64   `json:"win_rate"`
	AverageClearingPrice           float64   `json:"average_clearing_price"`
	Impressions                    int64     `json:"impressions"`
	BudgetLimitedImpressions       int64     `json:"budget_limited_impressions"`
	IncrementalReach               int64     `json:"incremental_reach"`
	AveragePCTR                    float64   `json:"average_pctr"`
	Clicks                         int64     `json:"clicks"`
	AveragePCVR                    float64   `json:"average_pcvr"`
	TrueConversions                int64     `json:"true_conversions"`
	ReportedConversions            int64     `json:"reported_conversions_after_backfill"`
	Spend                          float64   `json:"spend"`
	TrueRevenue                    float64   `json:"true_revenue"`
	ReportedRevenue                float64   `json:"reported_revenue_after_backfill"`
	AverageFrequency               float64   `json:"average_frequency"`
	AverageSaturationFactor        float64   `json:"average_saturation_factor"`
	AverageFatigueFactor           float64   `json:"average_fatigue_factor"`
	LearningProgress               float64   `json:"learning_progress"`
	ExpectedRemainingOpportunities int64     `json:"expected_remaining_opportunities"`
	PacingParticipationProbability float64   `json:"pacing_participation_probability"`
	AverageInternalCompetitors     float64   `json:"average_internal_competitors"`
	EligibilityReason              string    `json:"eligibility_reason,omitempty"`
}

type AdCalibration struct {
	DefaultBid       float64            `json:"default_bid"`
	CreativeQuality  float64            `json:"creative_quality"`
	Novelty          float64            `json:"novelty"`
	AudienceAffinity map[string]float64 `json:"audience_affinity"`
	CreativeAffinity map[string]float64 `json:"creative_affinity"`
	AssetID          string             `json:"asset_id"`
	ReviewState      string             `json:"review_state"`
}

func defaultAdCalibrations() map[string]AdCalibration {
	return map[string]AdCalibration{
		"ad_prospect_creator":  {1.85, 1.10, 1.08, map[string]float64{"broad_home": 1.08, "engaged_viewers": 1.12, "high_intent_cart": .92}, map[string]float64{"broad_home": 1.11, "engaged_viewers": 1.08, "high_intent_cart": .96}, "creative_creator_pov", "APPROVED"},
		"ad_prospect_demo":     {1.65, 1.02, .96, map[string]float64{"broad_home": 1.03, "engaged_viewers": .98, "high_intent_cart": .91}, map[string]float64{"broad_home": 1.01, "engaged_viewers": .98, "high_intent_cart": .95}, "creative_modular_demo", "APPROVED"},
		"ad_interest_room":     {1.70, .98, .86, map[string]float64{"broad_home": .95, "engaged_viewers": 1.12, "high_intent_cart": .94}, map[string]float64{"broad_home": .97, "engaged_viewers": 1.05, "high_intent_cart": .95}, "creative_room_reveal_v1", "APPROVED"},
		"ad_interest_before":   {1.60, 1.04, 1.02, map[string]float64{"broad_home": .94, "engaged_viewers": 1.10, "high_intent_cart": .92}, map[string]float64{"broad_home": 1.02, "engaged_viewers": 1.07, "high_intent_cart": .94}, "creative_entryway", "APPROVED"},
		"ad_lal_unboxing":      {1.95, 1.13, 1.03, map[string]float64{"broad_home": .93, "engaged_viewers": 1.12, "high_intent_cart": 1.18}, map[string]float64{"broad_home": 1.02, "engaged_viewers": 1.10, "high_intent_cart": 1.15}, "creative_unboxing", "APPROVED"},
		"ad_lal_review":        {1.82, 1.07, .83, map[string]float64{"broad_home": .90, "engaged_viewers": 1.08, "high_intent_cart": 1.16}, map[string]float64{"broad_home": .96, "engaged_viewers": 1.06, "high_intent_cart": 1.13}, "creative_customer_review_v1", "APPROVED"},
		"ad_viewers_founder":   {1.55, 1.04, .91, map[string]float64{"broad_home": .74, "engaged_viewers": 1.22, "high_intent_cart": 1.10}, map[string]float64{"broad_home": .90, "engaged_viewers": 1.08, "high_intent_cart": 1.04}, "creative_founder_materials", "APPROVED"},
		"ad_viewers_offer":     {1.72, 1.09, .98, map[string]float64{"broad_home": .72, "engaged_viewers": 1.24, "high_intent_cart": 1.20}, map[string]float64{"broad_home": .92, "engaged_viewers": 1.10, "high_intent_cart": 1.12}, "creative_shipping_offer", "APPROVED"},
		"ad_cart_proof":        {2.05, 1.15, 1.04, map[string]float64{"broad_home": .64, "engaged_viewers": 1.08, "high_intent_cart": 1.48}, map[string]float64{"broad_home": .86, "engaged_viewers": 1.05, "high_intent_cart": 1.20}, "creative_social_proof", "APPROVED"},
		"ad_cart_urgency":      {1.92, .94, .79, map[string]float64{"broad_home": .62, "engaged_viewers": 1.02, "high_intent_cart": 1.50}, map[string]float64{"broad_home": .82, "engaged_viewers": .96, "high_intent_cart": 1.14}, "creative_stock_message_v1", "LIMITED"},
		"ad_launch_teaser":     {1.60, 1.06, 1.06, map[string]float64{"broad_home": 1.02}, map[string]float64{"broad_home": 1.04}, "creative_fall_teaser_v1", "APPROVED"},
		"ad_launch_collection": {1.72, 1.10, 1.02, map[string]float64{"broad_home": 1.05}, map[string]float64{"broad_home": 1.08}, "creative_fall_montage", "APPROVED"},
	}
}

func newSimulationModel(config SimulationConfig) SimulationModelState {
	return SimulationModelState{Version: causalModelVersion, Config: config, LifetimeSpend: map[string]float64{}, ReportExposure: map[string]map[string]int64{}, AudienceCohorts: map[string]CohortDeliveryState{}, CreativeCohorts: map[string]CreativeCohortDeliveryState{}, Creatives: map[string]CreativeDeliveryState{}, Learning: map[string]LearningState{}, DailySpend: map[string]float64{}}
}

func (s SimulationModelState) validate() error {
	if s.Version != causalModelVersion || s.LifetimeSpend == nil || s.ReportExposure == nil || s.AudienceCohorts == nil || s.CreativeCohorts == nil || s.Creatives == nil || s.Learning == nil || s.DailySpend == nil {
		return errors.New("invalid sandbox causal model state")
	}
	if err := s.Config.validate(); err != nil {
		return err
	}
	for _, state := range s.AudienceCohorts {
		if state.CumulativeImpressions < 0 || state.UniqueReachedUsers < 0 {
			return errors.New("invalid sandbox audience delivery state")
		}
	}
	for _, state := range s.CreativeCohorts {
		if state.CumulativeImpressions < 0 || state.UniqueReachedUsers < 0 {
			return errors.New("invalid sandbox creative delivery state")
		}
	}
	return nil
}

func (f *Backend) currentAdCalibration(ad ads.Entity) AdCalibration {
	value, ok := f.model.Config.Ads[ad.ID]
	if !ok {
		value = AdCalibration{DefaultBid: 1.7, CreativeQuality: 1, Novelty: 1, AudienceAffinity: map[string]float64{}, CreativeAffinity: map[string]float64{}, AssetID: "created_asset", ReviewState: "APPROVED"}
	}
	if update, exists := f.operations.Ads[ad.ID]; exists && update.AssetID != "" && update.AssetID != value.AssetID {
		value.AssetID = update.AssetID
		value.CreativeQuality = 1
		value.Novelty = 1.12
		value.ReviewState = "APPROVED"
	}
	return value
}

func (f *Backend) adGroupBid(group ads.Entity, profile AdCalibration) float64 {
	if v, ok := f.operations.AdGroups[group.ID]; ok && v.Bid != nil {
		return math.Max(.01, v.Bid.InexactFloat64())
	}
	return profile.DefaultBid
}

func (f *Backend) eligibility(ad, group, campaign ads.Entity, cohort Cohort, hour time.Time) (bool, string) {
	if campaign.Status != "ENABLE" || group.Status != "ENABLE" || ad.Status != "ENABLE" {
		return false, "delivery_status"
	}
	settings := f.operations.AdGroups[group.ID]
	definition := f.operations.AdGroupDefinitions[group.ID]
	if len(definition.AgeGroups) > 0 && !contains(definition.AgeGroups, cohort.AgeGroup) {
		return false, "age_targeting"
	}
	if definition.Gender != "" && definition.Gender != "GENDER_UNLIMITED" && !strings.EqualFold(definition.Gender, cohort.Gender) {
		return false, "gender_targeting"
	}
	if settings.ScheduleStart != "" {
		start, ok := f.parseDeliveryTime(settings.ScheduleStart)
		if !ok {
			return false, "schedule_invalid"
		}
		if hour.Before(start) {
			return false, "schedule_not_started"
		}
	}
	if settings.ScheduleEnd != "" {
		end, ok := f.parseDeliveryTime(settings.ScheduleEnd)
		if !ok {
			return false, "schedule_invalid"
		}
		if hour.After(end) {
			return false, "schedule_ended"
		}
	}
	if len(settings.LocationIDs) > 0 && !contains(settings.LocationIDs, cohort.Geo) {
		return false, "geo_targeting"
	}
	if !contains(f.model.Config.EligiblePlacements, cohort.Placement) {
		return false, "placement_inventory"
	}
	if len(settings.Placements) > 0 && !placementsMatch(settings.Placements, cohort.Placement) {
		return false, "placement_targeting"
	}
	if len(settings.Languages) > 0 && !contains(settings.Languages, cohort.Language) {
		return false, "language_targeting"
	}
	if len(settings.AudienceIDs) > 0 {
		matched := false
		for _, id := range settings.AudienceIDs {
			matched = matched || f.audienceMatches(id, cohort)
		}
		if !matched {
			return false, "audience_definition"
		}
	}
	for _, id := range settings.ExcludedAudienceIDs {
		if f.audienceMatches(id, cohort) {
			return false, "audience_exclusion"
		}
	}
	profile := f.currentAdCalibration(ad)
	if profile.AssetID == "" {
		return false, "creative_missing"
	}
	if profile.ReviewState != "APPROVED" && profile.ReviewState != "LIMITED" {
		return false, "policy_review"
	}
	if f.model.Config.Availability <= 0 {
		return false, "inventory_unavailable"
	}
	if f.remainingBudget(group, campaign, hour) <= f.model.Config.FloorImpressionCost {
		return false, "budget_exhausted"
	}
	return true, ""
}

func (f *Backend) parseDeliveryTime(value string) (time.Time, bool) {
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC(), true
	}
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, f.location)
	return parsed.UTC(), err == nil
}

func placementsMatch(configured []string, cohort string) bool {
	for _, placement := range configured {
		if strings.EqualFold(placement, cohort) {
			return true
		}
		if strings.EqualFold(placement, "PLACEMENT_TIKTOK") && strings.EqualFold(cohort, "FEED") {
			return true
		}
	}
	return false
}

func audienceMatchesCohort(audienceID, cohortID string) bool {
	switch audienceID {
	case "audience_prospecting":
		return cohortID == "broad_home" || cohortID == "test"
	case "audience_purchasers":
		return cohortID == "high_intent_cart"
	case "audience_lookalike":
		return cohortID == "broad_home" || cohortID == "engaged_viewers" || cohortID == "test"
	default:
		// Unknown definitions cannot establish membership.
		return false
	}
}

func contains(values []string, wanted string) bool {
	for _, v := range values {
		if strings.EqualFold(v, wanted) {
			return true
		}
	}
	return false
}

func (f *Backend) dailyBudget(entity ads.Entity) float64 {
	if entity.Budget == nil {
		return math.Inf(1)
	}
	value := entity.Budget.InexactFloat64()
	if entity.BudgetMode == "BUDGET_MODE_TOTAL" {
		// A lifetime budget has no independent daily cap.
		return math.Inf(1)
	}
	return value
}

func (f *Backend) remainingBudget(group, campaign ads.Entity, hour time.Time) float64 {
	groupRemaining := f.remainingEntityBudget(group, hour)
	campaignRemaining := f.remainingEntityBudget(campaign, hour)
	return math.Max(0, math.Min(groupRemaining, campaignRemaining))
}

func (f *Backend) recordSpend(group, campaign ads.Entity, hour time.Time, spend float64) {
	day := hour.In(f.location).Format(time.DateOnly)
	f.model.DailySpend[day+"/"+group.ID] += spend
	f.model.DailySpend[day+"/"+campaign.ID] += spend
	f.model.LifetimeSpend[group.ID] += spend
	f.model.LifetimeSpend[campaign.ID] += spend
	// The ledger only needs the current and prior day for pacing and replay state.
	cutoff := hour.In(f.location).AddDate(0, 0, -2).Format(time.DateOnly)
	for key := range f.model.DailySpend {
		if len(key) >= 10 && key[:10] < cutoff {
			delete(f.model.DailySpend, key)
		}
	}
}

func seededRand(seed int64, values ...string) *rand.Rand {
	h := sha256.New()
	var raw [8]byte
	binary.BigEndian.PutUint64(raw[:], uint64(seed))
	_, _ = h.Write(raw[:])
	for _, value := range values {
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(value))
	}
	sum := h.Sum(nil)
	return rand.New(rand.NewSource(int64(binary.BigEndian.Uint64(sum[:8]))))
}

func clamp(value, low, high float64) float64 { return math.Max(low, math.Min(high, value)) }

func binomial(rng *rand.Rand, n int64, p float64) int64 {
	p = clamp(p, 0, 1)
	if n <= 0 || p == 0 {
		return 0
	}
	if p == 1 {
		return n
	}
	if n <= 1024 {
		var out int64
		for i := int64(0); i < n; i++ {
			if rng.Float64() < p {
				out++
			}
		}
		return out
	}
	mean := float64(n) * p
	std := math.Sqrt(float64(n) * p * (1 - p))
	return int64(math.Round(clamp(mean+rng.NormFloat64()*std, 0, float64(n))))
}

func poisson(rng *rand.Rand, mean float64) int {
	if mean > 30 {
		return maxInt(1, int(math.Round(mean+rng.NormFloat64()*math.Sqrt(mean))))
	}
	limit, product, n := math.Exp(-mean), 1.0, 0
	for product > limit {
		product *= rng.Float64()
		n++
	}
	return maxInt(1, n-1)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (f *Backend) marketMultipliers(hour time.Time) (traffic, intent, count, bid float64) {
	traffic, intent, count, bid = 1, 1, 1, 1
	for _, event := range f.model.Config.MarketEvents {
		if !hour.Before(event.Start) && !hour.After(event.End) {
			traffic *= event.TrafficMultiplier
			intent *= event.PurchaseIntentMultiplier
			count *= event.CompetitorCountMultiplier
			bid *= event.CompetitorBidMultiplier
		}
	}
	return
}

func (f *Backend) learningFor(group ads.Entity) LearningState {
	settings := f.operations.AdGroups[group.ID]
	creativeIDs := []string{}
	groupAds := []ads.Entity{}
	for _, entity := range f.entities {
		if entity.Level == ads.Ad && entity.ParentID == group.ID {
			groupAds = append(groupAds, entity)
		}
	}
	sort.Slice(groupAds, func(i, j int) bool { return groupAds[i].ID < groupAds[j].ID })
	for _, entity := range groupAds {
		creativeIDs = append(creativeIDs, entity.ID+":"+f.currentAdCalibration(entity).AssetID)
	}
	sort.Strings(creativeIDs)
	fingerprint := fingerprint(settings.ScheduleStart, settings.ScheduleEnd, settings.Placements, settings.AudienceIDs, settings.ExcludedAudienceIDs, settings.LocationIDs, settings.Languages, creativeIDs)
	budget := f.dailyBudget(group)
	profile := AdCalibration{DefaultBid: 1.7}
	if len(groupAds) > 0 {
		profile = f.currentAdCalibration(groupAds[0])
	}
	bid := f.adGroupBid(group, profile)
	state := f.model.Learning[group.ID]
	if state.Fingerprint == "" {
		state.Fingerprint = fingerprint
		state.Budget = budget
		state.Bid = bid
	} else if state.Fingerprint != fingerprint || majorRatio(state.Budget, budget) >= f.model.Config.LearningMajorChangeRatio || majorRatio(state.Bid, bid) >= f.model.Config.LearningMajorChangeRatio {
		// A material configuration change partially resets accumulated learning.
		state.Fingerprint = fingerprint
		state.Clicks = min64(int64(float64(state.Clicks)*.35), int64(f.model.Config.LearningClickThreshold*.45))
		state.Conversions = math.Min(state.Conversions*.35, f.model.Config.LearningConversionThreshold*.45)
	}
	state.Budget = budget
	state.Bid = bid
	config := f.model.Config
	state.Progress = clamp(.5*float64(state.Clicks)/config.LearningClickThreshold+.5*state.Conversions/config.LearningConversionThreshold, 0, 1)
	f.model.Learning[group.ID] = state
	return state
}

func majorRatio(before, after float64) float64 {
	if before <= 0 || math.IsInf(before, 0) || math.IsInf(after, 0) {
		if before == after {
			return 0
		}
		return 1
	}
	return math.Abs(after-before) / before
}

func (f *Backend) updateLearning(groupID string, clicks, conversions int64) {
	state := f.model.Learning[groupID]
	state.Clicks += clicks
	state.Conversions += float64(conversions)
	config := f.model.Config
	state.Progress = clamp(.5*float64(state.Clicks)/config.LearningClickThreshold+.5*state.Conversions/config.LearningConversionThreshold, 0, 1)
	f.model.Learning[groupID] = state
}

func defaultOne(value float64) float64 {
	if value == 0 {
		return 1
	}
	return value
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func addSimulationMetrics(m ads.Metrics, spend float64, impressions, clicks, conversions int64, revenue float64, reach, landing, view2, view6, complete int64) ads.Metrics {
	m.Spend = m.Spend.Add(decimal.NewFromFloat(spend))
	m.Impressions += impressions
	m.Clicks += clicks
	conversionValue := decimal.NewFromInt(conversions)
	if m.Conversions == nil {
		m.Conversions = &conversionValue
	} else {
		v := m.Conversions.Add(conversionValue)
		m.Conversions = &v
	}
	revenueValue := decimal.NewFromFloat(revenue)
	if m.Revenue == nil {
		m.Revenue = &revenueValue
	} else {
		v := m.Revenue.Add(revenueValue)
		m.Revenue = &v
	}
	m.Reach = sumOptionalInt(m.Reach, reach)
	m.LandingPageViews = sumOptionalInt(m.LandingPageViews, landing)
	m.VideoViews2s = sumOptionalInt(m.VideoViews2s, view2)
	m.VideoViews6s = sumOptionalInt(m.VideoViews6s, view6)
	m.VideoViewsComplete = sumOptionalInt(m.VideoViewsComplete, complete)
	return m
}

func sumOptionalInt(current *int64, value int64) *int64 {
	if current != nil {
		value += *current
	}
	return &value
}

func orderValues(rng *rand.Rand, conversions int64, config SimulationConfig) (trueRevenue float64, reportedConversions int64, reportedRevenue float64) {
	for i := int64(0); i < conversions; i++ {
		value := math.Exp(config.AOVLogMean + config.AOVLogSigma*rng.NormFloat64())
		trueRevenue += value
		if rng.Float64() < config.TrackingCoverage {
			reportedConversions++
			reportedRevenue += value
		}
	}
	return
}

// ConfigureSimulation is a developer/test seam. It is unavailable after virtual time
// has advanced and is not exposed through HTTP or Agent tools.
func (f *Backend) ConfigureSimulation(config SimulationConfig) error {
	if err := config.validate(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.hourFacts) > 0 {
		return errors.New("simulation calibration is immutable after advance")
	}
	f.model = newSimulationModel(config)
	rows, err := f.historicalSeed(14)
	if err != nil {
		return err
	}
	f.rows = rows
	return nil
}

// DebugTraces returns hidden causal state only to in-process developer/test callers.
// Normal Ad Agent tools and HTTP routes have no access to this method.
func (f *Backend) DebugTraces(start, end time.Time) []CausalTrace {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := []CausalTrace{}
	for _, fact := range f.hourFacts {
		if fact.Trace != nil && !fact.Hour.Before(start) && !fact.Hour.After(end) {
			out = append(out, *fact.Trace)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Hour.Equal(out[j].Hour) {
			return out[i].AdID < out[j].AdID
		}
		return out[i].Hour.Before(out[j].Hour)
	})
	return out
}

// SetSimulationDebug controls trace capture for developer tooling. It does not alter
// delivery calculations and is not exposed through AdBackend or Agent tools.
func (f *Backend) SetSimulationDebug(enabled bool) {
	f.mu.Lock()
	f.model.Config.Debug = enabled
	f.mu.Unlock()
}
