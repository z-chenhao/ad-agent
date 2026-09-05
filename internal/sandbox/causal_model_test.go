package sandbox

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/z-chenhao/ad-agent/internal/ads"
)

type scenarioStats struct {
	opportunities, participations, impressions, reach, clicks, conversions, reportedConversions int64
	spend, revenue, pctr, pcvr, competitor, clearing, frequency                                 float64
}

func scenario(t *testing.T, mutate func(*Backend, *SimulationConfig), hours int) scenarioStats {
	t.Helper()
	b, err := NewEnvironment("causal-test")
	if err != nil {
		t.Fatal(err)
	}
	config := DefaultSimulationConfig()
	config.Debug = true
	config.Cohorts = []Cohort{{ID: "test", Geo: "US", Language: "en", Placement: "FEED", Population: 260000, DailyActiveRate: .72, AudienceAffinity: 1.12, PurchaseIntent: 1.2, BaseCTR: .018, BaseCVR: .045, MarketCompetition: 1, ReachableUsers: 260000}}
	profile := config.Ads["ad_prospect_creator"]
	profile.AudienceAffinity = map[string]float64{"test": 1}
	profile.CreativeAffinity = map[string]float64{"test": 1}
	config.Ads["ad_prospect_creator"] = profile
	ad := b.entities["ad_prospect_creator"]
	group := b.entities[ad.ParentID]
	campaign := b.entities[group.ParentID]
	large := decimal.NewFromInt(100000)
	group.Budget, group.BudgetMode = &large, "BUDGET_MODE_DAY"
	campaign.Budget, campaign.BudgetMode = &large, "BUDGET_MODE_DAY"
	b.entities[group.ID], b.entities[campaign.ID] = group, campaign
	mutate(b, &config)
	if err := config.validate(); err != nil {
		t.Fatal(err)
	}
	b.model = newSimulationModel(config)
	b.rows = nil
	var out scenarioStats
	for i := 1; i <= hours; i++ {
		fact := b.generateCausalHour(ad, b.simulationStart.Add(time.Duration(i)*time.Hour))
		trace := fact.Trace
		if trace == nil {
			t.Fatal("debug trace missing")
		}
		out.opportunities += trace.Opportunities
		out.participations += trace.AuctionParticipations
		out.impressions += trace.Impressions
		out.reach += trace.IncrementalReach
		out.clicks += trace.Clicks
		out.conversions += trace.TrueConversions
		out.reportedConversions += trace.ReportedConversions
		out.spend += trace.Spend
		out.revenue += trace.TrueRevenue
		out.pctr += trace.AveragePCTR * float64(trace.Impressions)
		out.pcvr += trace.AveragePCVR * float64(trace.Clicks)
		out.competitor += trace.AverageCompetitorScore
		out.clearing += trace.AverageClearingPrice
		out.frequency = trace.AverageFrequency
	}
	if out.impressions > 0 {
		out.pctr /= float64(out.impressions)
	}
	if out.clicks > 0 {
		out.pcvr /= float64(out.clicks)
	}
	out.competitor /= float64(hours)
	out.clearing /= float64(hours)
	return out
}

func TestCausalModelClassifiesInvariantsAssumptionsAndCalibration(t *testing.T) {
	found := map[AssumptionClass]bool{}
	for _, statement := range ModelStatements() {
		found[statement.Class] = true
	}
	for _, class := range []AssumptionClass{PlatformInvariant, ModelingAssumption, CalibrationParameter} {
		if !found[class] {
			t.Fatalf("missing model statement class %s", class)
		}
	}
}

func TestBidRaisesWinRateWithDiminishingReturns(t *testing.T) {
	run := func(bid float64) scenarioStats {
		return scenario(t, func(_ *Backend, c *SimulationConfig) {
			profile := c.Ads["ad_prospect_creator"]
			profile.DefaultBid = bid
			c.Ads["ad_prospect_creator"] = profile
		}, 96)
	}
	low, medium, high := run(.45), run(2.0), run(8.0)
	lowRate := float64(low.impressions) / float64(low.participations)
	mediumRate := float64(medium.impressions) / float64(medium.participations)
	highRate := float64(high.impressions) / float64(high.participations)
	if !(lowRate < mediumRate && mediumRate < highRate) {
		t.Fatalf("bid did not raise win rate: low=%f medium=%f high=%f", lowRate, mediumRate, highRate)
	}
	if !(mediumRate-lowRate > highRate-mediumRate) {
		t.Fatalf("bid response lacks diminishing returns: low=%f medium=%f high=%f", lowRate, mediumRate, highRate)
	}
}

func TestTrafficObjectiveUsesClickValueAndStillDelivers(t *testing.T) {
	stats := scenario(t, func(b *Backend, _ *SimulationConfig) {
		campaign := b.entities["campaign_prospect_us"]
		campaign.Objective = "TRAFFIC"
		b.entities[campaign.ID] = campaign
	}, 48)
	if stats.participations == 0 || stats.impressions == 0 || stats.clicks == 0 {
		t.Fatalf("traffic objective did not produce a click-valued auction path: %+v", stats)
	}
}

func TestEligibilityUsesStartEndPlacementLanguageAndTargeting(t *testing.T) {
	hour := time.Date(2026, 9, 4, 16, 0, 0, 0, time.UTC)
	run := func(settings ads.AdGroupUpdateSpec) *CausalTrace {
		b, err := NewEnvironment("eligibility")
		if err != nil {
			t.Fatal(err)
		}
		config := DefaultSimulationConfig()
		config.Debug = true
		config.Cohorts = []Cohort{{ID: "test", Geo: "US", Language: "en", Placement: "FEED", Population: 260000, DailyActiveRate: .72, AudienceAffinity: 1.12, PurchaseIntent: 1.2, BaseCTR: .018, BaseCVR: .045, MarketCompetition: 1, ReachableUsers: 260000}}
		profile := config.Ads["ad_prospect_creator"]
		profile.AudienceAffinity = map[string]float64{"test": 1}
		profile.CreativeAffinity = map[string]float64{"test": 1}
		config.Ads["ad_prospect_creator"] = profile
		b.model = newSimulationModel(config)
		settings.AdGroupID = "adgroup_broad_us"
		b.operations.AdGroups[settings.AdGroupID] = settings
		return b.generateCausalHour(b.entities["ad_prospect_creator"], hour).Trace
	}
	if trace := run(ads.AdGroupUpdateSpec{ScheduleStart: hour.Add(time.Hour).Format(time.RFC3339)}); trace.EligibilityReason != "schedule_not_started" || trace.Impressions != 0 {
		t.Fatalf("future schedule delivered: %+v", trace)
	}
	if trace := run(ads.AdGroupUpdateSpec{ScheduleEnd: hour.Add(-time.Hour).Format(time.RFC3339)}); trace.EligibilityReason != "schedule_ended" || trace.Impressions != 0 {
		t.Fatalf("ended schedule delivered: %+v", trace)
	}
	if trace := run(ads.AdGroupUpdateSpec{Placements: []string{"PLACEMENT_PANGLE"}}); trace.EligibilityReason != "placement_targeting" || trace.Impressions != 0 {
		t.Fatalf("placement mismatch delivered: %+v", trace)
	}
	if trace := run(ads.AdGroupUpdateSpec{Placements: []string{"PLACEMENT_TIKTOK"}, Languages: []string{"zh"}}); trace.EligibilityReason != "language_targeting" || trace.Impressions != 0 {
		t.Fatalf("language mismatch delivered: %+v", trace)
	}
	if trace := run(ads.AdGroupUpdateSpec{ScheduleStart: hour.Add(-time.Hour).Format(time.RFC3339), ScheduleEnd: hour.Add(time.Hour).Format(time.RFC3339), Placements: []string{"PLACEMENT_TIKTOK"}, LocationIDs: []string{"US"}, Languages: []string{"en"}}); trace.EligibilityReason != "" || trace.EligibleOpportunities == 0 {
		t.Fatalf("eligible delivery path rejected: %+v", trace)
	}
}

func TestPacingUsesExpectedRemainingOpportunities(t *testing.T) {
	hour := time.Date(2026, 9, 4, 15, 0, 0, 0, time.UTC)
	run := func(end time.Time) *CausalTrace {
		b, err := NewEnvironment("forecast-pacing")
		if err != nil {
			t.Fatal(err)
		}
		config := DefaultSimulationConfig()
		config.Debug = true
		config.Cohorts = []Cohort{{ID: "test", Geo: "US", Language: "en", Placement: "FEED", Population: 260000, DailyActiveRate: .72, AudienceAffinity: 1.12, PurchaseIntent: 1.2, BaseCTR: .018, BaseCVR: .045, MarketCompetition: 1, ReachableUsers: 260000}}
		profile := config.Ads["ad_prospect_creator"]
		profile.AudienceAffinity = map[string]float64{"test": 1}
		profile.CreativeAffinity = map[string]float64{"test": 1}
		config.Ads["ad_prospect_creator"] = profile
		b.model = newSimulationModel(config)
		budget := decimal.NewFromInt(20)
		group := b.entities["adgroup_broad_us"]
		group.Budget = &budget
		b.entities[group.ID] = group
		b.operations.AdGroups[group.ID] = ads.AdGroupUpdateSpec{AdGroupID: group.ID, ScheduleEnd: end.Format(time.RFC3339)}
		return b.generateCausalHour(b.entities["ad_prospect_creator"], hour).Trace
	}
	short := run(hour.Add(2 * time.Hour))
	long := run(hour.Add(15 * time.Hour))
	if !(short.ExpectedRemainingOpportunities < long.ExpectedRemainingOpportunities && short.PacingParticipationProbability > long.PacingParticipationProbability) {
		t.Fatalf("pacing ignored remaining supply: short=%+v long=%+v", short, long)
	}
}

func TestJointAuctionIsOrderIndependentAndCreatesCannibalization(t *testing.T) {
	setup := func() *Backend {
		b, err := NewEnvironment("joint-auction")
		if err != nil {
			t.Fatal(err)
		}
		config := DefaultSimulationConfig()
		config.Debug = true
		config.Cohorts = []Cohort{{ID: "test", Geo: "US", Language: "en", Placement: "FEED", Population: 180000, DailyActiveRate: .70, AudienceAffinity: 1.1, PurchaseIntent: 1.2, BaseCTR: .018, BaseCVR: .045, MarketCompetition: .8, ReachableUsers: 180000}}
		for _, id := range []string{"ad_prospect_creator", "ad_interest_room"} {
			profile := config.Ads[id]
			profile.AudienceAffinity = map[string]float64{"test": 1}
			profile.CreativeAffinity = map[string]float64{"test": 1}
			config.Ads[id] = profile
		}
		b.model = newSimulationModel(config)
		large := decimal.NewFromInt(100000)
		for _, id := range []string{"campaign_prospect_us", "adgroup_broad_us", "adgroup_interest_home"} {
			entity := b.entities[id]
			entity.Budget = &large
			b.entities[id] = entity
		}
		return b
	}
	hour := time.Date(2026, 9, 4, 16, 0, 0, 0, time.UTC)
	forward, reverse := setup(), setup()
	a := forward.entities["ad_prospect_creator"]
	b := forward.entities["ad_interest_room"]
	forwardFacts := forward.generateCausalHoursJoint([]ads.Entity{a, b}, hour)
	reverseFacts := reverse.generateCausalHoursJoint([]ads.Entity{b, a}, hour)
	forwardJSON, _ := json.Marshal(forwardFacts)
	reverseJSON, _ := json.Marshal(reverseFacts)
	if string(forwardJSON) != string(reverseJSON) {
		t.Fatal("joint auction depends on input order")
	}
	opportunities := forwardFacts[0].Trace.Opportunities
	totalImpressions := forwardFacts[0].Metrics.Impressions + forwardFacts[1].Metrics.Impressions
	if totalImpressions > opportunities {
		t.Fatalf("one opportunity produced multiple advertiser impressions: opportunities=%d impressions=%d", opportunities, totalImpressions)
	}
	if forwardFacts[0].Trace.AverageInternalCompetitors == 0 && forwardFacts[1].Trace.AverageInternalCompetitors == 0 {
		t.Fatal("overlapping ad groups never entered the same auction")
	}
	single, joint := setup(), setup()
	var singleImpressions, jointImpressions int64
	for offset := 0; offset < 24; offset++ {
		instant := hour.Add(time.Duration(offset) * time.Hour)
		singleFact := single.generateCausalHour(single.entities[a.ID], instant)
		singleImpressions += singleFact.Metrics.Impressions
		jointFacts := joint.generateCausalHoursJoint([]ads.Entity{joint.entities[a.ID], joint.entities[b.ID]}, instant)
		jointImpressions += jointFacts[0].Metrics.Impressions
	}
	if jointImpressions >= singleImpressions {
		t.Fatalf("overlapping ad group did not cannibalize delivery: single=%d joint=%d", singleImpressions, jointImpressions)
	}
}

func TestWithinGroupCreativesShareOneAuctionEntry(t *testing.T) {
	b, err := NewEnvironment("creative-selection")
	if err != nil {
		t.Fatal(err)
	}
	config := DefaultSimulationConfig()
	config.Debug = true
	config.Cohorts = []Cohort{{ID: "test", Geo: "US", Language: "en", Placement: "FEED", Population: 180000, DailyActiveRate: .7, AudienceAffinity: 1.1, PurchaseIntent: 1.2, BaseCTR: .018, BaseCVR: .045, MarketCompetition: .8, ReachableUsers: 180000}}
	strong := config.Ads["ad_prospect_creator"]
	strong.CreativeQuality = 1.35
	strong.AudienceAffinity, strong.CreativeAffinity = map[string]float64{"test": 1}, map[string]float64{"test": 1}
	weak := config.Ads["ad_prospect_demo"]
	weak.CreativeQuality = .72
	weak.AudienceAffinity, weak.CreativeAffinity = map[string]float64{"test": 1}, map[string]float64{"test": 1}
	config.Ads["ad_prospect_creator"], config.Ads["ad_prospect_demo"] = strong, weak
	b.model = newSimulationModel(config)
	large := decimal.NewFromInt(100000)
	for _, id := range []string{"campaign_prospect_us", "adgroup_broad_us"} {
		entity := b.entities[id]
		entity.Budget = &large
		b.entities[id] = entity
	}
	var strongImpressions, weakImpressions int64
	for offset := 0; offset < 48; offset++ {
		hour := time.Date(2026, 9, 4, 8, 0, 0, 0, time.UTC).Add(time.Duration(offset) * time.Hour)
		facts := b.generateCausalHoursJoint([]ads.Entity{b.entities["ad_prospect_creator"], b.entities["ad_prospect_demo"]}, hour)
		if facts[0].Metrics.Impressions+facts[1].Metrics.Impressions > facts[0].Trace.Opportunities {
			t.Fatal("one ad group entered an opportunity more than once")
		}
		if facts[0].Trace.AverageInternalCompetitors != 0 || facts[1].Trace.AverageInternalCompetitors != 0 {
			t.Fatal("creatives in one ad group were treated as separate account bidders")
		}
		strongImpressions += facts[0].Metrics.Impressions
		weakImpressions += facts[1].Metrics.Impressions
	}
	if strongImpressions <= weakImpressions {
		t.Fatalf("internal creative selection ignored auction value: strong=%d weak=%d", strongImpressions, weakImpressions)
	}
}

func TestJointBudgetAllocationCapsSharedCampaign(t *testing.T) {
	b, err := NewEnvironment("shared-campaign-budget")
	if err != nil {
		t.Fatal(err)
	}
	config := DefaultSimulationConfig()
	config.Cohorts = []Cohort{{ID: "test", Geo: "US", Language: "en", Placement: "FEED", Population: 220000, DailyActiveRate: .72, AudienceAffinity: 1.1, PurchaseIntent: 1.2, BaseCTR: .018, BaseCVR: .045, MarketCompetition: .8, ReachableUsers: 220000}}
	for _, id := range []string{"ad_prospect_creator", "ad_interest_room"} {
		profile := config.Ads[id]
		profile.AudienceAffinity, profile.CreativeAffinity = map[string]float64{"test": 1}, map[string]float64{"test": 1}
		config.Ads[id] = profile
	}
	b.model = newSimulationModel(config)
	campaignBudget := decimal.NewFromInt(20)
	large := decimal.NewFromInt(100000)
	campaign := b.entities["campaign_prospect_us"]
	campaign.Budget = &campaignBudget
	b.entities[campaign.ID] = campaign
	for _, id := range []string{"adgroup_broad_us", "adgroup_interest_home"} {
		group := b.entities[id]
		group.Budget = &large
		b.entities[id] = group
	}
	var spend decimal.Decimal
	for offset := 0; offset < 24; offset++ {
		hour := time.Date(2026, 9, 4, 7, 0, 0, 0, time.UTC).Add(time.Duration(offset) * time.Hour)
		facts := b.generateCausalHoursJoint([]ads.Entity{b.entities["ad_prospect_creator"], b.entities["ad_interest_room"]}, hour)
		spend = spend.Add(facts[0].Metrics.Spend).Add(facts[1].Metrics.Spend)
	}
	if spend.GreaterThan(campaignBudget) {
		t.Fatalf("shared campaign spend exceeded budget: spend=%s budget=%s", spend, campaignBudget)
	}
}

func TestBudgetRaisesParticipationAndDeliveryUntilConstrained(t *testing.T) {
	run := func(budget float64) scenarioStats {
		return scenario(t, func(b *Backend, _ *SimulationConfig) {
			group := b.entities["adgroup_broad_us"]
			value := decimal.NewFromFloat(budget)
			group.Budget = &value
			b.entities[group.ID] = group
		}, 24)
	}
	low, high := run(25), run(350)
	if !(low.participations < high.participations && low.impressions < high.impressions) {
		t.Fatalf("budget did not expand paced delivery: low=%+v high=%+v", low, high)
	}
	if low.spend > 25.0001 || high.spend > 350.0001 {
		t.Fatalf("spend exceeded budget: low=%f high=%f", low.spend, high.spend)
	}
}

func TestCampaignBudgetAlsoPacesAndCapsDelivery(t *testing.T) {
	run := func(budget float64) scenarioStats {
		return scenario(t, func(b *Backend, _ *SimulationConfig) {
			campaign := b.entities["campaign_prospect_us"]
			value := decimal.NewFromFloat(budget)
			campaign.Budget = &value
			b.entities[campaign.ID] = campaign
		}, 24)
	}
	low, high := run(25), run(350)
	if !(low.participations < high.participations && low.impressions < high.impressions) {
		t.Fatalf("campaign budget did not govern paced delivery: low=%+v high=%+v", low, high)
	}
	if low.spend > 25.0001 || high.spend > 350.0001 {
		t.Fatalf("campaign spend exceeded budget: low=%f high=%f", low.spend, high.spend)
	}
}

func TestCompetitionLowersWinRateAndRaisesClearingCost(t *testing.T) {
	run := func(multiplier float64) scenarioStats {
		return scenario(t, func(_ *Backend, c *SimulationConfig) {
			c.CompetitorCountMean *= multiplier
			c.CompetitorBidMean *= math.Sqrt(multiplier)
		}, 96)
	}
	low, high := run(.55), run(1.9)
	lowWin := float64(low.impressions) / float64(low.participations)
	highWin := float64(high.impressions) / float64(high.participations)
	lowCPM := low.spend / float64(low.impressions) * 1000
	highCPM := high.spend / float64(high.impressions) * 1000
	if !(highWin < lowWin && highCPM > lowCPM && high.competitor > low.competitor) {
		t.Fatalf("competition causality failed: low=%+v high=%+v cpm=%f/%f", low, high, lowCPM, highCPM)
	}
}

func TestCreativeQualityAffectsCTRAndAuctionValue(t *testing.T) {
	run := func(quality float64) scenarioStats {
		return scenario(t, func(_ *Backend, c *SimulationConfig) {
			profile := c.Ads["ad_prospect_creator"]
			profile.CreativeQuality = quality
			c.Ads["ad_prospect_creator"] = profile
		}, 72)
	}
	low, high := run(.72), run(1.28)
	if !(high.pctr > low.pctr && high.impressions > low.impressions) {
		t.Fatalf("creative quality did not improve pCTR and auction delivery: low=%+v high=%+v", low, high)
	}
}

func TestLandingPageQualityChangesCVRWithoutChangingPCTR(t *testing.T) {
	run := func(quality float64) scenarioStats {
		return scenario(t, func(_ *Backend, c *SimulationConfig) { c.LandingPageQuality = quality }, 72)
	}
	poor, strong := run(.55), run(1.25)
	if !(strong.pcvr > poor.pcvr) {
		t.Fatalf("landing page did not change pCVR: poor=%f strong=%f", poor.pcvr, strong.pcvr)
	}
	if math.Abs(strong.pctr-poor.pctr)/poor.pctr > .01 {
		t.Fatalf("landing page changed pCTR: poor=%f strong=%f", poor.pctr, strong.pctr)
	}
}

func TestPurchaseIntentRaisesCVR(t *testing.T) {
	run := func(intent float64) scenarioStats {
		return scenario(t, func(_ *Backend, c *SimulationConfig) { c.Cohorts[0].PurchaseIntent = intent }, 72)
	}
	low, high := run(.65), run(1.65)
	if !(high.pcvr > low.pcvr && high.conversions > low.conversions) {
		t.Fatalf("purchase intent did not improve conversion: low=%+v high=%+v", low, high)
	}
}

func TestTrackingLossChangesReportedButNotTrueConversions(t *testing.T) {
	full := scenario(t, func(_ *Backend, c *SimulationConfig) { c.TrackingCoverage = 1 }, 72)
	loss := scenario(t, func(_ *Backend, c *SimulationConfig) { c.TrackingCoverage = .35 }, 72)
	if full.conversions != loss.conversions || !(loss.reportedConversions < full.reportedConversions) {
		t.Fatalf("tracking changed true outcomes or not reported outcomes: full=%+v loss=%+v", full, loss)
	}
}

func TestReportingDelayTemporarilyHidesThenBackfillsConversions(t *testing.T) {
	b, err := NewEnvironment("report-delay")
	if err != nil {
		t.Fatal(err)
	}
	config := DefaultSimulationConfig()
	config.TrackingCoverage = 1
	config.AttributionDelayHours = 5
	config.ReportingLatencyHours = 2
	b.model = newSimulationModel(config)
	ad := b.entities["ad_prospect_creator"]
	var chosen HourFact
	for i := 1; i <= 24; i++ {
		fact := b.generateCausalHour(ad, b.simulationStart.Add(time.Duration(i)*time.Hour))
		if fact.Metrics.Conversions != nil && !fact.Metrics.Conversions.IsZero() {
			chosen = fact
			break
		}
	}
	if chosen.AdID == "" {
		t.Fatal("test did not produce an attributed conversion")
	}
	b.hourFacts = append(b.hourFacts, chosen)
	day := chosen.Hour.In(b.location).Format(time.DateOnly)
	b.account.LatestDate = day
	b.clock = chosen.Hour
	beforeReport, err := b.Report(context.Background(), ads.ReportQuery{Level: ads.Ad, EntityID: chosen.AdID, Start: day, End: day})
	if err != nil || len(beforeReport.Rows) != 1 {
		t.Fatalf("before report: %#v %v", beforeReport, err)
	}
	b.clock = chosen.ReportAvailableAt
	afterReport, err := b.Report(context.Background(), ads.ReportQuery{Level: ads.Ad, EntityID: chosen.AdID, Start: day, End: day})
	if err != nil || len(afterReport.Rows) != 1 {
		t.Fatalf("after report: %#v %v", afterReport, err)
	}
	before, after := beforeReport.Rows[0].Metrics, afterReport.Rows[0].Metrics
	if before.Conversions == nil || !before.Conversions.IsZero() || after.Conversions == nil || after.Conversions.IsZero() || !before.Spend.Equal(after.Spend) || before.Impressions != after.Impressions {
		t.Fatalf("reporting delay/backfill failed: before=%#v after=%#v", before, after)
	}
}

func TestSeasonalityActsThroughTrafficIntentAndCompetitors(t *testing.T) {
	baseline := scenario(t, func(_ *Backend, _ *SimulationConfig) {}, 72)
	shock := scenario(t, func(b *Backend, c *SimulationConfig) {
		c.MarketEvents = []MarketEvent{{Name: "retail_event", Start: b.simulationStart, End: b.simulationStart.Add(96 * time.Hour), TrafficMultiplier: 1.35, PurchaseIntentMultiplier: 1.25, CompetitorCountMultiplier: 1.55, CompetitorBidMultiplier: 1.30}}
	}, 72)
	baselineCPM := baseline.spend / float64(baseline.impressions) * 1000
	shockCPM := shock.spend / float64(shock.impressions) * 1000
	if !(shock.opportunities > baseline.opportunities && shock.pcvr > baseline.pcvr && shock.competitor > baseline.competitor && shockCPM > baselineCPM) {
		t.Fatalf("market shock bypassed causal inputs: baseline=%+v shock=%+v cpm=%f/%f", baseline, shock, baselineCPM, shockCPM)
	}
}

func TestMaterialBidChangePartiallyResetsLearning(t *testing.T) {
	b, err := NewEnvironment("learning-reset")
	if err != nil {
		t.Fatal(err)
	}
	config := DefaultSimulationConfig()
	config.Debug = true
	b.model = newSimulationModel(config)
	ad := b.entities["ad_prospect_creator"]
	for i := 1; i <= 96; i++ {
		b.generateCausalHour(ad, b.simulationStart.Add(time.Duration(i)*time.Hour))
	}
	groupID := ad.ParentID
	before := b.model.Learning[groupID].Progress
	minorBid := decimal.RequireFromString("1.95")
	b.operations.AdGroups[groupID] = ads.AdGroupUpdateSpec{AdGroupID: groupID, Bid: &minorBid}
	minor := b.generateCausalHour(ad, b.simulationStart.Add(97*time.Hour))
	if minor.Trace.LearningProgress < before*.95 {
		t.Fatalf("minor bid change reset learning: before=%f after=%f", before, minor.Trace.LearningProgress)
	}
	newBid := decimal.RequireFromString("4.50")
	b.operations.AdGroups[groupID] = ads.AdGroupUpdateSpec{AdGroupID: groupID, Bid: &newBid}
	fact := b.generateCausalHour(ad, b.simulationStart.Add(98*time.Hour))
	after := fact.Trace.LearningProgress
	if before <= 0 || !(after < before) {
		t.Fatalf("learning did not partially reset: before=%f after=%f", before, after)
	}
}

func TestCreativeReplacementUsesFreshExposureState(t *testing.T) {
	b, err := NewEnvironment("creative-reset")
	if err != nil {
		t.Fatal(err)
	}
	config := DefaultSimulationConfig()
	config.Debug = true
	config.Cohorts = []Cohort{{ID: "broad_home", Geo: "US", Language: "en", Placement: "FEED", Population: 260000, DailyActiveRate: .72, AudienceAffinity: 1.1, PurchaseIntent: 1.2, BaseCTR: .018, BaseCVR: .045, MarketCompetition: 1, ReachableUsers: 260000}}
	b.model = newSimulationModel(config)
	ad := b.entities["ad_prospect_creator"]
	profile := config.Ads[ad.ID]
	b.model.AudienceCohorts["broad_home"] = CohortDeliveryState{CumulativeImpressions: 120000, UniqueReachedUsers: 40000}
	b.model.CreativeCohorts[profile.AssetID+"/broad_home"] = CreativeCohortDeliveryState{CumulativeImpressions: 240000, UniqueReachedUsers: 40000}
	old := b.generateCausalHour(ad, b.simulationStart.Add(time.Hour))
	b.operations.Ads[ad.ID] = ads.AdCreativeUpdateSpec{AdID: ad.ID, AssetID: "creative_modular_demo"}
	fresh := b.generateCausalHour(ad, b.simulationStart.Add(2*time.Hour))
	if !(fresh.Trace.AverageFatigueFactor > old.Trace.AverageFatigueFactor && fresh.Trace.AveragePCTR > old.Trace.AveragePCTR) {
		t.Fatalf("replacement did not recover through fresh state: old=%+v fresh=%+v", old.Trace, fresh.Trace)
	}
	if math.Abs(fresh.Trace.AverageSaturationFactor-old.Trace.AverageSaturationFactor) > .03 {
		t.Fatalf("creative replacement reset audience saturation: old=%+v fresh=%+v", old.Trace, fresh.Trace)
	}
}

func TestFrequencyCausesFatigueAndSaturation(t *testing.T) {
	fresh := scenario(t, func(_ *Backend, _ *SimulationConfig) {}, 1)
	var fatigued scenarioStats
	// scenario resets model after mutate; construct the high-frequency state directly.
	b, _ := NewEnvironment("fatigue-direct")
	config := DefaultSimulationConfig()
	config.Debug = true
	config.Cohorts = []Cohort{{ID: "test", Geo: "US", Language: "en", Placement: "FEED", Population: 260000, DailyActiveRate: .72, AudienceAffinity: 1.12, PurchaseIntent: 1.2, BaseCTR: .018, BaseCVR: .045, MarketCompetition: 1, ReachableUsers: 260000}}
	profile := config.Ads["ad_prospect_creator"]
	profile.AudienceAffinity, profile.CreativeAffinity = map[string]float64{"test": 1}, map[string]float64{"test": 1}
	config.Ads["ad_prospect_creator"] = profile
	b.model = newSimulationModel(config)
	b.model.AudienceCohorts["test"] = CohortDeliveryState{CumulativeImpressions: 240000, UniqueReachedUsers: 40000}
	b.model.CreativeCohorts[profile.AssetID+"/test"] = CreativeCohortDeliveryState{CumulativeImpressions: 240000, UniqueReachedUsers: 40000}
	fact := b.generateCausalHour(b.entities["ad_prospect_creator"], b.simulationStart.Add(time.Hour))
	fatigued.pctr, fatigued.frequency = fact.Trace.AveragePCTR, fact.Trace.AverageFrequency
	if !(fatigued.frequency > fresh.frequency && fatigued.pctr < fresh.pctr && fact.Trace.AverageFatigueFactor < 1 && fact.Trace.AverageSaturationFactor < 1) {
		t.Fatalf("fatigue/saturation causality failed: fresh=%+v fatigued=%+v trace=%+v", fresh, fatigued, fact.Trace)
	}
}

func TestHigherBudgetEventuallyRaisesFrequencyAndSlowsIncrementalReach(t *testing.T) {
	run := func(budget float64) scenarioStats {
		return scenario(t, func(b *Backend, c *SimulationConfig) {
			c.Cohorts[0].Population, c.Cohorts[0].ReachableUsers = 52000, 42000
			group := b.entities["adgroup_broad_us"]
			value := decimal.NewFromFloat(budget)
			group.Budget = &value
			b.entities[group.ID] = group
		}, 24*10)
	}
	low, high := run(35), run(420)
	lowReachRate := float64(low.reach) / float64(low.impressions)
	highReachRate := float64(high.reach) / float64(high.impressions)
	if !(high.spend > low.spend && high.frequency > low.frequency && highReachRate < lowReachRate) {
		t.Fatalf("budget saturation failed: low=%+v high=%+v reach_rate=%f/%f", low, high, lowReachRate, highReachRate)
	}
}

func TestMetricsRemainDerivedFromEvents(t *testing.T) {
	stats := scenario(t, func(_ *Backend, _ *SimulationConfig) {}, 48)
	ctr := float64(stats.clicks) / float64(stats.impressions)
	cvr := float64(stats.conversions) / float64(stats.clicks)
	cpm := stats.spend / float64(stats.impressions) * 1000
	cpc := stats.spend / float64(stats.clicks)
	cpa := stats.spend / float64(stats.conversions)
	roas := stats.revenue / stats.spend
	for name, value := range map[string]float64{"ctr": ctr, "cvr": cvr, "cpm": cpm, "cpc": cpc, "cpa": cpa, "roas": roas} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
			t.Fatalf("derived %s is invalid: %f from %+v", name, value, stats)
		}
	}
}

func TestSimulationDebugTraceIsNotAnAdBackendCapability(t *testing.T) {
	b, err := NewEnvironment("debug-boundary")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.Advance(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if traces := b.DebugTraces(time.Time{}, time.Now().AddDate(10, 0, 0)); len(traces) != 0 {
		t.Fatal("debug traces were stored while debug mode was disabled")
	}
}
