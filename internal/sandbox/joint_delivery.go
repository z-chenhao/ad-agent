package sandbox

import (
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/z-chenhao/ad-agent/internal/ads"
)

// deliveryCandidate is one advertiser ad's opportunity to serve one cohort in
// one timestep. All candidates are resolved in the same sampled auctions before
// budget is allocated, so map or entity iteration order cannot buy inventory.
type deliveryCandidate struct {
	ad, group, campaign ads.Entity
	profile             AdCalibration
	cohort              Cohort
	learning            LearningState
	pctr, pcvr          float64
	predicted           float64
	quality, score, bid float64
	frequency           float64
	saturation, fatigue float64
	pacing              float64
	remainingForecast   int64
	selectedSamples     int64
	winSamples          int64
	clearingTotal       float64
	externalScoreTotal  float64
	externalSamples     int64
	internalTotal       int64
	desiredImpressions  int64
	finalImpressions    int64
	averageCost         float64
}

type sampledEntry struct {
	index int
	score float64
}

func (f *Backend) generateCausalHour(ad ads.Entity, hour time.Time) HourFact {
	facts := f.generateCausalHoursJoint([]ads.Entity{ad}, hour)
	if len(facts) == 0 {
		zero := ads.ZeroMetrics()
		return HourFact{AdID: ad.ID, Hour: hour.UTC(), ModelVersion: causalModelVersion, Metrics: zero, TrueMetrics: zero}
	}
	return facts[0]
}

func (f *Backend) generateCausalHoursJoint(adList []ads.Entity, hour time.Time) []HourFact {
	sort.Slice(adList, func(i, j int) bool { return adList[i].ID < adList[j].ID })
	facts := make(map[string]*HourFact, len(adList))
	traces := make(map[string]*CausalTrace, len(adList))
	for _, ad := range adList {
		zero := ads.ZeroMetrics()
		facts[ad.ID] = &HourFact{AdID: ad.ID, Hour: hour.UTC(), ModelVersion: causalModelVersion, Metrics: zero, TrueMetrics: zero}
		traces[ad.ID] = &CausalTrace{Hour: hour.UTC(), AdID: ad.ID}
	}

	groups, campaigns := f.deliveryHierarchy(adList)
	groupForecast := make(map[string]int64, len(groups))
	campaignForecast := make(map[string]int64, len(campaigns))
	for groupID, groupAds := range groups {
		group := f.entities[groupID]
		campaign := f.entities[group.ParentID]
		forecast := f.expectedRemainingOpportunities(groupAds, group, campaign, hour)
		groupForecast[groupID] = forecast
		campaignForecast[campaign.ID] += forecast
	}
	groupPacing := make(map[string]float64, len(groups))
	groupLearning := make(map[string]LearningState, len(groups))
	for groupID, groupAds := range groups {
		group := f.entities[groupID]
		campaign := f.entities[group.ParentID]
		learning := f.learningFor(group)
		groupLearning[groupID] = learning
		groupPacing[groupID] = f.pacingProbability(groupAds, group, campaign, hour, groupForecast[groupID], campaignForecast[campaign.ID])
	}

	candidates := make([]deliveryCandidate, 0, len(adList)*len(f.model.Config.Cohorts))
	for _, cohort := range f.model.Config.Cohorts {
		opportunities := f.opportunitiesAt(cohort, hour)
		cohortIndexes := make([]int, 0, len(adList))
		for _, ad := range adList {
			trace := traces[ad.ID]
			trace.Opportunities += opportunities
			group, groupOK := f.entities[ad.ParentID]
			campaign, campaignOK := f.entities[group.ParentID]
			if !groupOK || !campaignOK {
				trace.EligibilityReason = "hierarchy"
				continue
			}
			eligible, reason := f.eligibility(ad, group, campaign, cohort, hour)
			if !eligible {
				if trace.EligibilityReason == "" {
					trace.EligibilityReason = reason
				}
				continue
			}
			trace.EligibilityReason = ""
			trace.EligibleOpportunities += opportunities
			candidate := f.newDeliveryCandidate(ad, group, campaign, cohort, hour, groupLearning[group.ID], groupPacing[group.ID], groupForecast[group.ID])
			candidates = append(candidates, candidate)
			cohortIndexes = append(cohortIndexes, len(candidates)-1)
		}
		f.resolveCohortAuctions(candidates, cohortIndexes, opportunities, cohort, hour)
	}

	f.applyJointBudgetAllocation(candidates, hour)
	f.materializeCandidateOutcomes(candidates, facts, traces, hour)
	f.model.GeneratedSteps++

	out := make([]HourFact, 0, len(adList))
	for _, ad := range adList {
		fact := facts[ad.ID]
		trace := traces[ad.ID]
		finalizeTrace(trace)
		fact.Metrics.Spend = fact.Metrics.Spend.Truncate(4)
		fact.TrueMetrics.Spend = fact.TrueMetrics.Spend.Truncate(4)
		if fact.Metrics.Revenue != nil {
			value := fact.Metrics.Revenue.Round(2)
			fact.Metrics.Revenue = &value
		}
		if fact.TrueMetrics.Revenue != nil {
			value := fact.TrueMetrics.Revenue.Round(2)
			fact.TrueMetrics.Revenue = &value
		}
		fact.ReportAvailableAt = hour.Add(time.Duration(f.model.Config.AttributionDelayHours+f.model.Config.ReportingLatencyHours) * time.Hour).UTC()
		if f.model.Config.Debug {
			fact.Trace = trace
		}
		out = append(out, *fact)
	}
	return out
}

func (f *Backend) deliveryHierarchy(adList []ads.Entity) (map[string][]ads.Entity, map[string]ads.Entity) {
	groups := map[string][]ads.Entity{}
	campaigns := map[string]ads.Entity{}
	for _, ad := range adList {
		group, ok := f.entities[ad.ParentID]
		if !ok {
			continue
		}
		campaign, ok := f.entities[group.ParentID]
		if !ok {
			continue
		}
		groups[group.ID] = append(groups[group.ID], ad)
		campaigns[campaign.ID] = campaign
	}
	for id := range groups {
		sort.Slice(groups[id], func(i, j int) bool { return groups[id][i].ID < groups[id][j].ID })
	}
	return groups, campaigns
}

func (f *Backend) opportunitiesAt(cohort Cohort, hour time.Time) int64 {
	traffic, _, _, _ := f.marketMultipliers(hour)
	local := hour.In(f.location)
	daypart := .60
	if local.Hour() >= 8 && local.Hour() <= 22 {
		daypart = 1
	}
	return int64(math.Round(float64(cohort.Population) * cohort.DailyActiveRate / 24 * traffic * daypart))
}

func (f *Backend) expectedRemainingOpportunities(adList []ads.Entity, group, campaign ads.Entity, hour time.Time) int64 {
	local := hour.In(f.location)
	end := time.Date(local.Year(), local.Month(), local.Day(), 23, 0, 0, 0, f.location)
	var total int64
	for cursor := local.Truncate(time.Hour); !cursor.After(end); cursor = cursor.Add(time.Hour) {
		instant := cursor.UTC()
		for _, cohort := range f.model.Config.Cohorts {
			eligible := false
			for _, ad := range adList {
				if ok, _ := f.eligibility(ad, group, campaign, cohort, instant); ok {
					eligible = true
					break
				}
			}
			if eligible {
				total += f.opportunitiesAt(cohort, instant)
			}
		}
	}
	return total
}

func (f *Backend) pacingProbability(adList []ads.Entity, group, campaign ads.Entity, hour time.Time, groupForecast, campaignForecast int64) float64 {
	if groupForecast <= 0 {
		return 0
	}
	groupRemaining := f.remainingEntityBudget(group, hour)
	campaignRemaining := f.remainingEntityBudget(campaign, hour)
	if math.IsInf(groupRemaining, 1) && math.IsInf(campaignRemaining, 1) {
		return f.model.Config.MaxParticipation
	}
	campaignShare := campaignRemaining
	if !math.IsInf(campaignRemaining, 1) && campaignForecast > 0 {
		campaignShare = campaignRemaining * float64(groupForecast) / float64(campaignForecast)
	}
	available := math.Min(groupRemaining, campaignShare)
	if available <= f.model.Config.FloorImpressionCost {
		return 0
	}
	averageBid := 0.0
	for _, ad := range adList {
		averageBid += f.adGroupBid(group, f.currentAdCalibration(ad))
	}
	averageBid /= float64(len(adList))
	expectedCost := f.model.Config.FloorImpressionCost + averageBid*f.model.Config.BidToImpressionCost*.55
	fullParticipationSpend := float64(groupForecast) * f.model.Config.ExpectedWinRate * expectedCost
	base := available / math.Max(fullParticipationSpend, f.model.Config.FloorImpressionCost)
	local := hour.In(f.location)
	elapsed := (float64(local.Hour()) + 1) / 24
	dailyCapacity := math.Min(f.dailyBudget(group), f.dailyBudget(campaign))
	correction := 1.0
	if !math.IsInf(dailyCapacity, 1) && dailyCapacity > 0 {
		spent := f.model.DailySpend[local.Format(time.DateOnly)+"/"+group.ID]
		lag := (dailyCapacity*elapsed - spent) / dailyCapacity
		correction = math.Exp(f.model.Config.PacingSmoothness * lag)
	}
	return clamp(base*correction, 0, f.model.Config.MaxParticipation)
}

func (f *Backend) remainingEntityBudget(entity ads.Entity, hour time.Time) float64 {
	if entity.Budget != nil && entity.BudgetMode == "BUDGET_MODE_TOTAL" {
		return math.Max(0, entity.Budget.InexactFloat64()-f.model.LifetimeSpend[entity.ID])
	}
	day := hour.In(f.location).Format(time.DateOnly)
	return math.Max(0, f.dailyBudget(entity)-f.model.DailySpend[day+"/"+entity.ID])
}

func (f *Backend) newDeliveryCandidate(ad, group, campaign ads.Entity, cohort Cohort, hour time.Time, learning LearningState, pacing float64, forecast int64) deliveryCandidate {
	config := f.model.Config
	profile := f.currentAdCalibration(ad)
	audienceState := f.model.AudienceCohorts[cohort.ID]
	audienceFrequency := float64(audienceState.CumulativeImpressions) / math.Max(audienceState.UniqueReachedUsers, 1)
	saturation := math.Exp(-config.SaturationLambda * math.Max(audienceFrequency-config.SaturationThreshold, 0))
	creativeKey := profile.AssetID + "/" + cohort.ID
	creativeState := f.model.CreativeCohorts[creativeKey]
	creativeFrequency := float64(creativeState.CumulativeImpressions) / math.Max(creativeState.UniqueReachedUsers, 1)
	fatigue := math.Exp(-config.FatigueLambda * math.Max(creativeFrequency-config.FatigueThreshold, 0))
	creative := f.model.Creatives[profile.AssetID]
	if creative.AssetID != profile.AssetID {
		creative = CreativeDeliveryState{AssetID: profile.AssetID, FirstSeenHour: hour.Unix()}
		f.model.Creatives[profile.AssetID] = creative
	}
	ageHours := math.Max(0, float64(hour.Unix()-creative.FirstSeenHour)/3600)
	novelty := 1 + (profile.Novelty-1)*math.Exp(-ageHours/(24*5))
	rng := seededRand(config.Seed, f.environment, f.account.ID, ad.ID, cohort.ID, hour.UTC().Format(time.RFC3339), "context")
	contextSigma := .025
	contextualFactor := math.Exp(rng.NormFloat64()*contextSigma - .5*contextSigma*contextSigma)
	audienceAffinity := cohort.AudienceAffinity * defaultOne(profile.AudienceAffinity[cohort.ID])
	creativeAffinity := defaultOne(profile.CreativeAffinity[cohort.ID])
	pctr := clamp(cohort.BaseCTR*audienceAffinity*creativeAffinity*profile.CreativeQuality*novelty*fatigue*saturation*contextualFactor, .0001, .35)
	_, intentShock, _, _ := f.marketMultipliers(hour)
	pcvr := clamp(cohort.BaseCVR*audienceAffinity*cohort.PurchaseIntent*intentShock*config.ProductQuality*config.PriceAttractiveness*config.LandingPageQuality*config.Availability, .0001, .75)
	predictedPCTR := cohort.BaseCTR + learning.Progress*(pctr-cohort.BaseCTR)
	predictedPCVR := cohort.BaseCVR + learning.Progress*(pcvr-cohort.BaseCVR)
	predicted := predictedPCTR
	if strings.Contains(strings.ToUpper(campaign.Objective), "CONVERSION") {
		predicted = predictedPCTR * predictedPCVR * math.Exp(config.AOVLogMean+.5*config.AOVLogSigma*config.AOVLogSigma)
	} else {
		predicted *= config.ClickObjectiveValueScale
	}
	quality := clamp(profile.CreativeQuality*creativeAffinity, .25, 2.5)
	bid := f.adGroupBid(group, profile)
	// Generic behavioral abstraction only; this is not a reverse-engineered platform formula.
	score := math.Log1p(bid*config.BidUtilityScale) * predicted * quality
	return deliveryCandidate{ad: ad, group: group, campaign: campaign, profile: profile, cohort: cohort, learning: learning, pctr: pctr, pcvr: pcvr, predicted: predicted, quality: quality, score: score, bid: bid, frequency: audienceFrequency, saturation: saturation, fatigue: fatigue, pacing: pacing * saturation, remainingForecast: forecast}
}

func (f *Backend) resolveCohortAuctions(candidates []deliveryCandidate, indexes []int, opportunities int64, cohort Cohort, hour time.Time) {
	if len(indexes) == 0 || opportunities <= 0 {
		return
	}
	byGroup := map[string][]int{}
	for _, index := range indexes {
		byGroup[candidates[index].group.ID] = append(byGroup[candidates[index].group.ID], index)
	}
	groupIDs := make([]string, 0, len(byGroup))
	for id := range byGroup {
		groupIDs = append(groupIDs, id)
		sort.Ints(byGroup[id])
	}
	sort.Strings(groupIDs)
	rng := seededRand(f.model.Config.Seed, f.environment, f.account.ID, cohort.ID, hour.UTC().Format(time.RFC3339), "joint-auction")
	samples := int64(f.model.Config.AuctionSamples)
	if opportunities < samples {
		samples = opportunities
	}
	externalWins := int64(0)
	for sample := int64(0); sample < samples; sample++ {
		entries := make([]sampledEntry, 0, len(groupIDs))
		for _, groupID := range groupIDs {
			groupCandidates := byGroup[groupID]
			if rng.Float64() >= candidates[groupCandidates[0]].pacing {
				continue
			}
			selected := selectInternalCandidate(rng, candidates, groupCandidates, f.model.Config.InternalSelectionTemperature)
			candidate := &candidates[selected]
			candidate.selectedSamples++
			uncertainty := f.model.Config.ExplorationStrength * (1 - candidate.learning.Progress)
			observedScore := candidate.score * math.Exp(rng.NormFloat64()*uncertainty-.5*uncertainty*uncertainty)
			entries = append(entries, sampledEntry{index: selected, score: observedScore})
		}
		externalBest := f.sampleExternalBest(rng, cohort, hour)
		for _, entry := range entries {
			candidate := &candidates[entry.index]
			candidate.externalScoreTotal += externalBest
			candidate.externalSamples++
			candidate.internalTotal += int64(len(entries) - 1)
		}
		winner := -1
		winningScore, runnerScore := externalBest, 0.0
		for _, entry := range entries {
			if entry.score > winningScore {
				runnerScore = winningScore
				winningScore = entry.score
				winner = entry.index
			} else if entry.score > runnerScore {
				runnerScore = entry.score
			}
		}
		if winner < 0 {
			externalWins++
			continue
		}
		candidate := &candidates[winner]
		candidate.winSamples++
		requiredUtility := runnerScore / math.Max(candidate.predicted*candidate.quality, 1e-9)
		requiredBid := math.Expm1(requiredUtility) / f.model.Config.BidUtilityScale
		maxCost := f.model.Config.FloorImpressionCost + candidate.bid*f.model.Config.BidToImpressionCost
		cost := f.model.Config.FloorImpressionCost + requiredBid*f.model.Config.BidToImpressionCost
		candidate.clearingTotal += clamp(cost, f.model.Config.FloorImpressionCost, maxCost)
	}
	counts := make([]int64, len(indexes)+1)
	for position, index := range indexes {
		counts[position] = candidates[index].winSamples
	}
	counts[len(indexes)] = externalWins
	allocated := proportionalIntegerAllocation(opportunities, counts, samples)
	for position, index := range indexes {
		candidate := &candidates[index]
		candidate.desiredImpressions = allocated[position]
		candidate.finalImpressions = candidate.desiredImpressions
		candidate.selectedSamples = scaleSampleCount(opportunities, candidate.selectedSamples, samples)
		if candidate.winSamples > 0 {
			candidate.averageCost = candidate.clearingTotal / float64(candidate.winSamples)
		} else {
			candidate.averageCost = f.model.Config.FloorImpressionCost
		}
	}
}

func selectInternalCandidate(rng *rand.Rand, candidates []deliveryCandidate, indexes []int, temperature float64) int {
	if len(indexes) == 1 {
		return indexes[0]
	}
	maxScore := 0.0
	minimumLearning := 1.0
	for _, index := range indexes {
		maxScore = math.Max(maxScore, candidates[index].score)
		minimumLearning = math.Min(minimumLearning, candidates[index].learning.Progress)
	}
	exploration := .28 * (1 - minimumLearning)
	weights := make([]float64, len(indexes))
	total := 0.0
	for position, index := range indexes {
		exploit := math.Pow(math.Max(candidates[index].score/math.Max(maxScore, 1e-9), 1e-6), 1/temperature)
		weights[position] = (1-exploration)*exploit + exploration
		total += weights[position]
	}
	draw := rng.Float64() * total
	for position, weight := range weights {
		draw -= weight
		if draw <= 0 {
			return indexes[position]
		}
	}
	return indexes[len(indexes)-1]
}

func (f *Backend) sampleExternalBest(rng *rand.Rand, cohort Cohort, hour time.Time) float64 {
	_, _, countShock, bidShock := f.marketMultipliers(hour)
	n := poisson(rng, f.model.Config.CompetitorCountMean*cohort.MarketCompetition*countShock)
	best := 0.0
	for i := 0; i < n; i++ {
		bid := math.Exp(math.Log(f.model.Config.CompetitorBidMean*bidShock) + f.model.Config.CompetitorBidSigma*rng.NormFloat64())
		predicted := f.model.Config.CompetitorAuctionValueMean * clamp(1+.22*rng.NormFloat64(), .35, 1.8)
		quality := f.model.Config.CompetitorQualityMean * clamp(1+.18*rng.NormFloat64(), .45, 1.65)
		score := math.Log1p(bid*f.model.Config.BidUtilityScale) * predicted * quality
		best = math.Max(best, score)
	}
	return best
}

func proportionalIntegerAllocation(total int64, counts []int64, denominator int64) []int64 {
	out := make([]int64, len(counts))
	if total <= 0 || denominator <= 0 {
		return out
	}
	type remainder struct {
		index int
		value int64
	}
	remainders := make([]remainder, len(counts))
	allocated := int64(0)
	for index, count := range counts {
		numerator := total * count
		out[index] = numerator / denominator
		allocated += out[index]
		remainders[index] = remainder{index: index, value: numerator % denominator}
	}
	sort.SliceStable(remainders, func(i, j int) bool { return remainders[i].value > remainders[j].value })
	for extra := int64(0); extra < total-allocated; extra++ {
		out[remainders[int(extra)%len(remainders)].index]++
	}
	return out
}

func scaleSampleCount(total, count, samples int64) int64 {
	if samples <= 0 {
		return 0
	}
	return int64(math.Round(float64(total) * float64(count) / float64(samples)))
}

func (f *Backend) applyJointBudgetAllocation(candidates []deliveryCandidate, hour time.Time) {
	groupDesired := map[string]float64{}
	for index := range candidates {
		candidate := &candidates[index]
		groupDesired[candidate.group.ID] += float64(candidate.desiredImpressions) * candidate.averageCost
	}
	groupScale := map[string]float64{}
	for index := range candidates {
		candidate := &candidates[index]
		if _, exists := groupScale[candidate.group.ID]; exists {
			continue
		}
		groupScale[candidate.group.ID] = math.Min(1, f.remainingEntityBudget(candidate.group, hour)/math.Max(groupDesired[candidate.group.ID], 1e-9))
	}
	campaignDesired := map[string]float64{}
	for index := range candidates {
		candidate := &candidates[index]
		campaignDesired[candidate.campaign.ID] += float64(candidate.desiredImpressions) * candidate.averageCost * groupScale[candidate.group.ID]
	}
	campaignScale := map[string]float64{}
	for index := range candidates {
		candidate := &candidates[index]
		if _, exists := campaignScale[candidate.campaign.ID]; exists {
			continue
		}
		campaignScale[candidate.campaign.ID] = math.Min(1, f.remainingEntityBudget(candidate.campaign, hour)/math.Max(campaignDesired[candidate.campaign.ID], 1e-9))
	}
	for index := range candidates {
		candidate := &candidates[index]
		scale := groupScale[candidate.group.ID] * campaignScale[candidate.campaign.ID]
		candidate.finalImpressions = int64(math.Floor(float64(candidate.desiredImpressions) * scale))
	}
	// Prepaid balance is shared across all campaigns, without priority by iteration order.
	desired := 0.0
	for _, candidate := range candidates {
		desired += float64(candidate.finalImpressions) * candidate.averageCost
	}
	if desired > f.operations.Balance.Available.InexactFloat64() {
		scale := math.Max(0, f.operations.Balance.Available.InexactFloat64()) / math.Max(desired, 1e-9)
		for index := range candidates {
			candidates[index].finalImpressions = int64(math.Floor(float64(candidates[index].finalImpressions) * scale))
		}
	}
}

func (f *Backend) materializeCandidateOutcomes(candidates []deliveryCandidate, facts map[string]*HourFact, traces map[string]*CausalTrace, hour time.Time) {
	cohortIndexes := map[string][]int{}
	for index := range candidates {
		cohortIndexes[candidates[index].cohort.ID] = append(cohortIndexes[candidates[index].cohort.ID], index)
	}
	cohortReach := map[int]int64{}
	for cohortID, indexes := range cohortIndexes {
		totalImpressions := int64(0)
		for _, index := range indexes {
			totalImpressions += candidates[index].finalImpressions
		}
		state := f.model.AudienceCohorts[cohortID]
		cohort := candidates[indexes[0]].cohort
		remaining := math.Max(0, float64(cohort.ReachableUsers)-state.UniqueReachedUsers)
		incremental := int64(math.Round(math.Min(float64(totalImpressions), remaining*(1-math.Exp(-float64(totalImpressions)/math.Max(float64(cohort.ReachableUsers), 1))))))
		weights := make([]int64, len(indexes))
		for position, index := range indexes {
			weights[position] = candidates[index].finalImpressions
		}
		allocatedReach := proportionalIntegerAllocation(incremental, weights, max64(1, totalImpressions))
		for position, index := range indexes {
			cohortReach[index] = allocatedReach[position]
		}
		state.CumulativeImpressions += totalImpressions
		state.UniqueReachedUsers = math.Min(float64(cohort.ReachableUsers), state.UniqueReachedUsers+float64(incremental))
		f.model.AudienceCohorts[cohortID] = state
	}

	groupClicks, groupConversions := map[string]int64{}, map[string]int64{}
	for index := range candidates {
		candidate := &candidates[index]
		impressions := candidate.finalImpressions
		exposureKey := candidate.ad.ID + "/" + hour.In(f.location).Format(time.DateOnly)
		if f.model.ReportExposure[exposureKey] == nil {
			f.model.ReportExposure[exposureKey] = map[string]int64{}
		}
		f.model.ReportExposure[exposureKey][candidate.cohort.ID] += impressions
		rng := seededRand(f.model.Config.Seed, f.environment, f.account.ID, candidate.ad.ID, candidate.cohort.ID, hour.UTC().Format(time.RFC3339), "actions")
		clicks := binomial(rng, impressions, candidate.pctr)
		clickConversions := binomial(rng, clicks, candidate.pcvr)
		_, intentShock, _, _ := f.marketMultipliers(hour)
		viewConversions := binomial(rng, impressions-clicks, clamp(f.model.Config.ViewThroughRate*candidate.cohort.PurchaseIntent*intentShock, 0, .02))
		conversions := clickConversions + viewConversions
		trueRevenue, reportedConversions, reportedRevenue := orderValues(rng, conversions, f.model.Config)
		landing := binomial(rng, clicks, clamp(.76*f.model.Config.LandingPageQuality, 0, 1))
		view2 := binomial(rng, impressions, clamp(.62*candidate.profile.CreativeQuality, 0, .95))
		view6 := binomial(rng, view2, clamp(.53*candidate.profile.CreativeQuality, 0, .92))
		complete := binomial(rng, view6, clamp(.36*candidate.profile.CreativeQuality, 0, .85))
		spend := float64(impressions) * candidate.averageCost
		reach := cohortReach[index]
		fact := facts[candidate.ad.ID]
		fact.Metrics = addSimulationMetrics(fact.Metrics, spend, impressions, clicks, reportedConversions, reportedRevenue, reach, landing, view2, view6, complete)
		fact.TrueMetrics = addSimulationMetrics(fact.TrueMetrics, spend, impressions, clicks, conversions, trueRevenue, reach, landing, view2, view6, complete)
		fact.Attribution.ClickThrough += clickConversions
		fact.Attribution.ViewThrough += viewConversions
		trace := traces[candidate.ad.ID]
		trace.AuctionParticipations += candidate.selectedSamples
		trace.AuctionWins += candidate.desiredImpressions
		trace.Impressions += impressions
		trace.BudgetLimitedImpressions += candidate.desiredImpressions - impressions
		trace.IncrementalReach += reach
		trace.Clicks += clicks
		trace.TrueConversions += conversions
		trace.ReportedConversions += reportedConversions
		trace.Spend += spend
		trace.TrueRevenue += trueRevenue
		trace.ReportedRevenue += reportedRevenue
		trace.AveragePCTR += candidate.pctr * float64(impressions)
		trace.AveragePCVR += candidate.pcvr * float64(clicks)
		trace.AverageFrequency += candidate.frequency * float64(max64(1, impressions))
		trace.AverageSaturationFactor += candidate.saturation * float64(max64(1, impressions))
		trace.AverageFatigueFactor += candidate.fatigue * float64(max64(1, impressions))
		trace.AverageClearingPrice += candidate.averageCost * float64(impressions)
		if candidate.externalSamples > 0 {
			trace.AverageCompetitorScore += candidate.externalScoreTotal / float64(candidate.externalSamples) * float64(candidate.selectedSamples)
			trace.AverageInternalCompetitors += float64(candidate.internalTotal) / float64(candidate.externalSamples) * float64(candidate.selectedSamples)
		}
		trace.ExpectedRemainingOpportunities = max64(trace.ExpectedRemainingOpportunities, candidate.remainingForecast)
		trace.PacingParticipationProbability = math.Max(trace.PacingParticipationProbability, candidate.pacing)
		trace.LearningProgress = math.Max(trace.LearningProgress, candidate.learning.Progress)
		groupClicks[candidate.group.ID] += clicks
		groupConversions[candidate.group.ID] += conversions
		f.recordSpend(candidate.group, candidate.campaign, hour, spend)
		creativeKey := candidate.profile.AssetID + "/" + candidate.cohort.ID
		creativeState := f.model.CreativeCohorts[creativeKey]
		remainingCreativeReach := math.Max(0, float64(candidate.cohort.ReachableUsers)-creativeState.UniqueReachedUsers)
		creativeReach := math.Min(float64(impressions), remainingCreativeReach*(1-math.Exp(-float64(impressions)/math.Max(float64(candidate.cohort.ReachableUsers), 1))))
		creativeState.CumulativeImpressions += impressions
		creativeState.UniqueReachedUsers = math.Min(float64(candidate.cohort.ReachableUsers), creativeState.UniqueReachedUsers+creativeReach)
		f.model.CreativeCohorts[creativeKey] = creativeState
		creative := f.model.Creatives[candidate.profile.AssetID]
		creative.CumulativeImpressions += impressions
		f.model.Creatives[candidate.profile.AssetID] = creative
	}
	for groupID, clicks := range groupClicks {
		f.updateLearning(groupID, clicks, groupConversions[groupID])
	}
}

func finalizeTrace(trace *CausalTrace) {
	if trace.AuctionParticipations > 0 {
		trace.WinRate = float64(trace.AuctionWins) / float64(trace.AuctionParticipations)
	}
	if trace.Impressions > 0 {
		denominator := float64(trace.Impressions)
		trace.AveragePCTR /= denominator
		trace.AverageFrequency /= denominator
		trace.AverageSaturationFactor /= denominator
		trace.AverageFatigueFactor /= denominator
		trace.AverageClearingPrice /= denominator
	}
	if trace.Clicks > 0 {
		trace.AveragePCVR /= float64(trace.Clicks)
	}
	if trace.AuctionParticipations > 0 {
		trace.AverageCompetitorScore /= float64(trace.AuctionParticipations)
		trace.AverageInternalCompetitors /= float64(trace.AuctionParticipations)
	}
}
