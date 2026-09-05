package sandbox

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/shopspring/decimal"
	"github.com/z-chenhao/ad-agent/internal/ads"
)

// OperationState is the persisted mutable resource layer of one Sandbox
// environment. Historical delivery facts remain immutable in their own store.
type OperationState struct {
	AudienceDefinitions map[string]ads.AudienceCreateSpec      `json:"audience_definitions"`
	RuleDefinitions     map[string]ads.AutomatedRuleCreateSpec `json:"rule_definitions"`
	RuleResults         []ads.AutomatedRuleResult              `json:"rule_results"`
	RuleEvaluatedAt     map[string]time.Time                   `json:"rule_evaluated_at"`
	AdGroupDefinitions  map[string]ads.AdGroupSpec             `json:"ad_group_definitions"`
	Audiences           map[string]ads.Audience                `json:"audiences"`
	Rules               map[string]ads.AutomatedRule           `json:"rules"`
	EventSources        map[string]ads.EventSource             `json:"event_sources"`
	Comments            map[string]ads.Comment                 `json:"comments"`
	AdGroups            map[string]ads.AdGroupUpdateSpec       `json:"ad_group_settings"`
	Ads                 map[string]ads.AdCreativeUpdateSpec    `json:"ad_creative_settings"`
	Balance             ads.BillingBalance                     `json:"balance"`
	Transactions        []ads.BillingTransaction               `json:"transactions"`
}

func defaultOperationState(account ads.Account, now time.Time) OperationState {
	return OperationState{
		AudienceDefinitions: map[string]ads.AudienceCreateSpec{}, RuleDefinitions: map[string]ads.AutomatedRuleCreateSpec{}, RuleEvaluatedAt: map[string]time.Time{}, AdGroupDefinitions: map[string]ads.AdGroupSpec{},
		Audiences: map[string]ads.Audience{}, Rules: map[string]ads.AutomatedRule{},
		EventSources: map[string]ads.EventSource{}, AdGroups: map[string]ads.AdGroupUpdateSpec{},
		Ads: map[string]ads.AdCreativeUpdateSpec{},
		Comments: map[string]ads.Comment{
			"comment_shipping": {ID: "comment_shipping", AccountID: account.ID, AdID: "ad_prospect_creator", TikTokItemID: "item_warm_loft", Author: "Maya R.", Text: "Does this ship to California, and how long does assembly take?", Status: "VISIBLE", CreatedAt: now.Add(-47 * time.Minute), ReplyCount: 0},
			"comment_material": {ID: "comment_material", AccountID: account.ID, AdID: "ad_lal_unboxing", TikTokItemID: "item_vintage_shelves", Author: "Jordan K.", Text: "Is the wood solid or veneer?", Status: "VISIBLE", CreatedAt: now.Add(-3 * time.Hour), ReplyCount: 1},
			"comment_spam":     {ID: "comment_spam", AccountID: account.ID, AdID: "ad_interest_room", TikTokItemID: "item_modern_living", Author: "promo_deals", Text: "DM for cheap followers", Status: "VISIBLE", CreatedAt: now.Add(-8 * time.Hour), ReplyCount: 0},
		},
		Balance: ads.BillingBalance{AccountID: account.ID, Currency: account.Currency, Available: decimal.RequireFromString("7421.80"), Cash: decimal.RequireFromString("6921.80"), Voucher: decimal.RequireFromString("500.00"), AsOf: now},
		Transactions: []ads.BillingTransaction{
			{ID: "txn_20260830", AccountID: account.ID, Type: "TOP_UP", Amount: decimal.RequireFromString("5000.00"), Currency: account.Currency, OccurredAt: now.Add(-96 * time.Hour), Status: "SUCCEEDED"},
			{ID: "txn_20260827", AccountID: account.ID, Type: "AD_SPEND", Amount: decimal.RequireFromString("-1834.62"), Currency: account.Currency, OccurredAt: now.Add(-7 * 24 * time.Hour), Status: "SETTLED"},
		},
	}
}

func (f *Backend) OperationState() OperationState {
	f.mu.RLock()
	defer f.mu.RUnlock()
	b, _ := json.Marshal(f.operations)
	var out OperationState
	_ = json.Unmarshal(b, &out)
	return out
}

func (f *Backend) RestoreOperationState(state OperationState) error {
	if state.Audiences == nil || state.Rules == nil || state.EventSources == nil || state.Comments == nil || state.AdGroups == nil || state.Ads == nil {
		return errors.New("invalid sandbox operation state")
	}
	if state.Balance.AccountID != f.account.ID || state.Balance.Currency != f.account.Currency {
		return errors.New("sandbox operation state account mismatch")
	}
	f.mu.Lock()
	if state.AudienceDefinitions == nil {
		state.AudienceDefinitions = map[string]ads.AudienceCreateSpec{}
	}
	if state.RuleDefinitions == nil {
		state.RuleDefinitions = map[string]ads.AutomatedRuleCreateSpec{}
	}
	if state.RuleEvaluatedAt == nil {
		state.RuleEvaluatedAt = map[string]time.Time{}
	}
	if state.AdGroupDefinitions == nil {
		state.AdGroupDefinitions = map[string]ads.AdGroupSpec{}
	}
	for id, rule := range state.Rules {
		if _, ok := state.RuleDefinitions[id]; !ok {
			rule.Status = "DISABLED"
			state.Rules[id] = rule
		}
	}
	f.operations = state
	f.mu.Unlock()
	return nil
}

func (f *Backend) AllEntities() []ads.Entity {
	f.mu.RLock()
	defer f.mu.RUnlock()
	values := make([]ads.Entity, 0, len(f.entities))
	for _, value := range f.entities {
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	return values
}

func fingerprint(values ...any) string {
	b, _ := json.Marshal(values)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func randomID(prefix string) string {
	var value [12]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(err)
	}
	return prefix + "_sandbox_" + hex.EncodeToString(value[:])
}

func (f *Backend) PrepareOperation(ctx context.Context, request ads.OperationRequest) (ads.OperationPlan, error) {
	if err := ctx.Err(); err != nil {
		return ads.OperationPlan{}, err
	}
	if err := request.Validate(); err != nil {
		return ads.OperationPlan{}, err
	}
	plan := ads.OperationPlan{Request: request, Lines: []ads.ChangeLine{}}
	var reviewBefore any
	var preconditions []any
	switch request.Kind {
	case ads.CreateCampaignBundle:
		v := request.CampaignBundle
		identities, _ := f.ListIdentities(ctx)
		assets, _ := f.ListCreativeAssets(ctx)
		identitySet, assetSet := map[string]bool{}, map[string]bool{}
		for _, item := range identities {
			identitySet[item.ID] = item.Status == "ACTIVE" || item.Status == "AUTHORIZED"
		}
		for _, item := range assets {
			assetSet[item.ID] = (item.Status == "AVAILABLE" || item.Status == "READY") && item.ReviewStatus == "APPROVED"
		}
		for _, ad := range v.Ads {
			if !identitySet[ad.IdentityID] || !assetSet[ad.AssetID] {
				return ads.OperationPlan{}, errors.New("campaign bundle references an unavailable identity or asset")
			}
		}
		if v.AdGroup.PixelID != "" {
			found := false
			for _, source := range mustSources(f, ctx) {
				found = found || source.ID == v.AdGroup.PixelID && source.Status == "ACTIVE"
			}
			if !found {
				return ads.OperationPlan{}, errors.New("campaign bundle references an unavailable event source")
			}
		}
		for _, id := range append(append([]string{}, v.AdGroup.AudienceIDs...), v.AdGroup.ExcludedAudienceIDs...) {
			if _, err := f.GetAudience(ctx, id); err != nil {
				return ads.OperationPlan{}, errors.New("campaign bundle references an unavailable audience")
			}
		}
		if err := f.validateTargeting(ctx, v.AdGroup.LocationIDs, v.AdGroup.Languages, v.AdGroup.Placements); err != nil {
			return ads.OperationPlan{}, err
		}
		preconditions = []any{identities, assets, mustSources(f, ctx), v.AdGroup.AudienceIDs, v.AdGroup.ExcludedAudienceIDs}
		plan.SpendIncreasing = false
	case ads.UpdateAdGroup:
		v := request.AdGroupUpdate
		entity, err := f.Get(ctx, ads.AdGroup, v.AdGroupID)
		if err != nil {
			return ads.OperationPlan{}, err
		}
		current := f.OperationState().AdGroups[v.AdGroupID]
		current.Budget = entity.Budget
		reviewBefore = ads.OperationRequest{Kind: ads.UpdateAdGroup, AdGroupUpdate: &current}
		for _, id := range append(append([]string{}, v.AudienceIDs...), v.ExcludedAudienceIDs...) {
			if _, err := f.GetAudience(ctx, id); err != nil {
				return ads.OperationPlan{}, errors.New("ad group update references an unavailable audience")
			}
		}
		if err := f.validateTargeting(ctx, v.LocationIDs, v.Languages, v.Placements); err != nil {
			return ads.OperationPlan{}, err
		}
		if v.Budget != nil {
			plan.SpendIncreasing = entity.Budget != nil && v.Budget.GreaterThan(*entity.Budget)
		}
		preconditions = []any{entity, current}
	case ads.UpdateAdCreative:
		v := request.AdUpdate
		detail, err := f.GetAdDetail(ctx, v.AdID)
		if err != nil {
			return ads.OperationPlan{}, err
		}
		if v.IdentityID != "" {
			found := false
			for _, i := range mustIdentities(f, ctx) {
				found = found || i.ID == v.IdentityID && (i.Status == "ACTIVE" || i.Status == "AUTHORIZED")
			}
			if !found {
				return ads.OperationPlan{}, errors.New("identity unavailable")
			}
		}
		if v.AssetID != "" {
			asset, err := f.GetCreativeAsset(ctx, v.AssetID)
			if err != nil || asset.ReviewStatus != "APPROVED" {
				return ads.OperationPlan{}, errors.New("creative asset unavailable")
			}
		}
		reviewBefore = ads.CreativeReviewBefore(detail)
		preconditions = []any{detail}
	case ads.CreateAudience:
		v := request.Audience
		if v.Kind == "lookalike" {
			source, err := f.GetAudience(ctx, v.SourceAudienceID)
			if err != nil || source.Kind != "custom" || source.Status != "READY" {
				return ads.OperationPlan{}, errors.New("lookalike source audience unavailable")
			}
			preconditions = []any{source}
		}
	case ads.CreateAutomatedRule:
		v := request.Rule
		for _, id := range v.TargetIDs {
			entity, err := f.Get(ctx, v.TargetLevel, id)
			if err != nil {
				return ads.OperationPlan{}, err
			}
			preconditions = append(preconditions, entity)
		}
		plan.SpendIncreasing = v.Action == "CHANGE_BUDGET"
	case ads.ModerateComment:
		v := request.Comment
		comment, err := f.comment(v.CommentID)
		if err != nil {
			return ads.OperationPlan{}, err
		}
		if comment.AdID != v.AdID || comment.TikTokItemID != v.TikTokItemID {
			return ads.OperationPlan{}, errors.New("comment scope mismatch")
		}
		preconditions = []any{comment}
	case ads.CreateEventSource:
	default:
		return ads.OperationPlan{}, errors.New("unsupported sandbox operation")
	}
	plan.Lines = ads.OperationReviewLines(request, reviewBefore)
	plan.PreconditionHash = fingerprint(preconditions...)
	return plan, nil
}

func (f *Backend) ApplyOperation(ctx context.Context, plan ads.OperationPlan) ads.OperationOutcome {
	if err := ctx.Err(); err != nil {
		return ads.OperationOutcome{State: "not_sent", Message: "request_cancelled"}
	}
	prepared, err := f.PrepareOperation(ctx, plan.Request)
	if err != nil || prepared.Version() != plan.Version() {
		return ads.OperationOutcome{State: "not_sent", Message: "operation_revalidation_failed"}
	}
	result := ads.OperationOutcome{State: "acknowledged", RequestIDs: []string{"sandbox-operation"}}
	switch plan.Request.Kind {
	case ads.CreateCampaignBundle:
		v := plan.Request.CampaignBundle
		campaign, err := f.Create(ctx, ads.CreateRequest{Level: ads.Campaign, Name: v.Campaign.Name, Status: "DISABLE", Budget: v.Campaign.Budget, BudgetMode: v.Campaign.BudgetMode, Objective: v.Campaign.Objective})
		if err != nil {
			return rejected(err)
		}
		group, err := f.Create(ctx, ads.CreateRequest{Level: ads.AdGroup, ParentID: campaign.ID, Name: v.AdGroup.Name, Status: "DISABLE", Budget: &v.AdGroup.Budget, BudgetMode: v.AdGroup.BudgetMode})
		if err != nil {
			return partial(result, err, campaign)
		}
		f.mu.Lock()
		f.operations.AdGroupDefinitions[group.ID] = v.AdGroup
		f.operations.AdGroups[group.ID] = ads.AdGroupUpdateSpec{AdGroupID: group.ID, Bid: v.AdGroup.Bid, ScheduleStart: v.AdGroup.ScheduleStart, ScheduleEnd: v.AdGroup.ScheduleEnd, Placements: v.AdGroup.Placements, AudienceIDs: v.AdGroup.AudienceIDs, ExcludedAudienceIDs: v.AdGroup.ExcludedAudienceIDs, LocationIDs: v.AdGroup.LocationIDs, Languages: v.AdGroup.Languages}
		f.mu.Unlock()
		result.Resources = append(result.Resources, resource(campaign), resource(group))
		for _, spec := range v.Ads {
			ad, createErr := f.Create(ctx, ads.CreateRequest{Level: ads.Ad, ParentID: group.ID, Name: spec.Name, Status: "DISABLE"})
			if createErr != nil {
				result.State = "partial"
				result.Message = "campaign bundle partially created"
				return result
			}
			f.mu.Lock()
			f.operations.Ads[ad.ID] = ads.AdCreativeUpdateSpec{AdID: ad.ID, IdentityID: spec.IdentityID, IdentityType: spec.IdentityType, AssetID: spec.AssetID, AssetKind: spec.AssetKind, PrimaryText: spec.PrimaryText, CallToAction: spec.CallToAction, DestinationURL: spec.DestinationURL}
			f.mu.Unlock()
			result.Resources = append(result.Resources, resource(ad))
		}
	case ads.UpdateAdGroup:
		v := *plan.Request.AdGroupUpdate
		if v.Budget != nil {
			entity, _ := f.Get(ctx, ads.AdGroup, v.AdGroupID)
			outcome := f.Write(ctx, ads.WriteRequest{Target: entity, Kind: string(ads.BudgetChange), Budget: v.Budget})
			if outcome.State != "acknowledged" {
				return ads.OperationOutcome{State: outcome.State, RequestIDs: []string{outcome.RequestID}, Message: outcome.Message}
			}
		}
		f.mu.Lock()
		current := f.operations.AdGroups[v.AdGroupID]
		mergeAdGroup(&current, v)
		f.operations.AdGroups[v.AdGroupID] = current
		f.mu.Unlock()
		result.Resources = []ads.OperationResource{{Kind: "ad_group", ID: v.AdGroupID}}
	case ads.UpdateAdCreative:
		v := *plan.Request.AdUpdate
		f.mu.Lock()
		current := f.operations.Ads[v.AdID]
		mergeAdCreative(&current, v)
		f.operations.Ads[v.AdID] = current
		f.mu.Unlock()
		result.Resources = []ads.OperationResource{{Kind: "ad", ID: v.AdID}}
	case ads.CreateAudience:
		v := plan.Request.Audience
		id := randomID("audience")
		size := int64(0)
		item := ads.Audience{ID: id, AccountID: f.account.ID, Name: v.Name, Kind: v.Kind, Status: "READY", ApproximateSize: &size, Source: v.SourceAudienceID, UpdatedAt: f.clock, PrivacyLimited: v.Kind == "lookalike"}
		f.mu.Lock()
		f.operations.AudienceDefinitions[id] = *v
		for _, cohort := range f.model.Config.Cohorts {
			if f.audienceMatches(id, cohort) {
				size += cohort.ReachableUsers
			}
		}
		f.operations.Audiences[id] = item
		f.mu.Unlock()
		result.Resources = []ads.OperationResource{{Kind: "audience", ID: id, Name: v.Name}}
	case ads.CreateAutomatedRule:
		v := plan.Request.Rule
		id := randomID("rule")
		item := ads.AutomatedRule{ID: id, AccountID: f.account.ID, Name: v.Name, Status: "ENABLED", TargetLevel: v.TargetLevel, Action: v.Action, Schedule: v.Schedule}
		f.mu.Lock()
		f.operations.RuleDefinitions[id] = *v
		f.operations.RuleEvaluatedAt[id] = f.clock
		f.operations.Rules[id] = item
		f.mu.Unlock()
		result.Resources = []ads.OperationResource{{Kind: "automated_rule", ID: id, Name: v.Name}}
	case ads.ModerateComment:
		v := plan.Request.Comment
		f.mu.Lock()
		item := f.operations.Comments[v.CommentID]
		switch v.Action {
		case "reply":
			item.ReplyCount++
		case "hide":
			item.Status = "HIDDEN"
		case "unhide":
			item.Status = "VISIBLE"
		case "delete":
			item.Status = "DELETED"
		}
		f.operations.Comments[v.CommentID] = item
		f.mu.Unlock()
		result.Resources = []ads.OperationResource{{Kind: "comment", ID: v.CommentID}}
	case ads.CreateEventSource:
		v := plan.Request.EventSource
		id := randomID(v.Kind)
		item := ads.EventSource{ID: id, AccountID: f.account.ID, Name: v.Name, Kind: v.Kind, Status: "ACTIVE", EventTypes: append([]string{}, v.EventTypes...)}
		f.mu.Lock()
		f.operations.EventSources[id] = item
		f.mu.Unlock()
		result.Resources = []ads.OperationResource{{Kind: "event_source", ID: id, Name: v.Name}}
	}
	return result
}

func (f *Backend) validateTargeting(ctx context.Context, locations, languages, placements []string) error {
	for kind, requested := range map[string][]string{"location": locations, "language": languages, "placement": placements} {
		if len(requested) == 0 {
			continue
		}
		options, err := f.ListTargetingOptions(ctx, kind)
		if err != nil {
			return err
		}
		available := make(map[string]bool, len(options))
		for _, option := range options {
			available[option.ID] = option.Enabled
		}
		for _, id := range requested {
			if !available[id] {
				return errors.New("operation references an unavailable " + kind)
			}
		}
	}
	return nil
}

func (f *Backend) ReconcileOperation(ctx context.Context, plan ads.OperationPlan, outcome ads.OperationOutcome) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if outcome.State != "acknowledged" || len(outcome.Resources) == 0 {
		return false, nil
	}
	if plan.Request.Kind == ads.CreateCampaignBundle {
		return f.reconcileBundle(ctx, *plan.Request.CampaignBundle, outcome)
	}
	state := f.OperationState()
	for _, item := range outcome.Resources {
		switch item.Kind {
		case "campaign":
			_, err := f.Get(ctx, ads.Campaign, item.ID)
			if err != nil {
				return false, nil
			}
		case "ad_group":
			entity, err := f.Get(ctx, ads.AdGroup, item.ID)
			if err != nil {
				return false, nil
			}
			if plan.Request.Kind == ads.UpdateAdGroup {
				current, ok := state.AdGroups[item.ID]
				current.Budget = entity.Budget
				if !ok || item.ID != plan.Request.AdGroupUpdate.AdGroupID || !reviewMatches(plan.Request, ads.OperationRequest{Kind: ads.UpdateAdGroup, AdGroupUpdate: &current}) {
					return false, nil
				}
			}
		case "ad":
			_, err := f.Get(ctx, ads.Ad, item.ID)
			if err != nil {
				return false, nil
			}
			current, ok := state.Ads[item.ID]
			if !ok || plan.Request.AdUpdate == nil || item.ID != plan.Request.AdUpdate.AdID || !reviewMatches(plan.Request, ads.OperationRequest{Kind: ads.UpdateAdCreative, AdUpdate: &current}) {
				return false, nil
			}
		case "audience":
			if _, ok := state.Audiences[item.ID]; !ok || !equalJSON(state.AudienceDefinitions[item.ID], *plan.Request.Audience) {
				return false, nil
			}
		case "automated_rule":
			if _, ok := state.Rules[item.ID]; !ok || !equalJSON(state.RuleDefinitions[item.ID], *plan.Request.Rule) {
				return false, nil
			}
		case "comment":
			if _, ok := state.Comments[item.ID]; !ok {
				return false, nil
			}
		case "event_source":
			actual, ok := state.EventSources[item.ID]
			want := plan.Request.EventSource
			if !ok || want == nil || actual.Name != want.Name || actual.Kind != want.Kind || !equalJSON(actual.EventTypes, append([]string{}, want.EventTypes...)) {
				return false, nil
			}
		default:
			return false, nil
		}
	}
	return true, nil
}

func (f *Backend) ListComments(ctx context.Context, adID string, limit int) ([]ads.Comment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		return nil, errors.New("comment limit must be 1-100")
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := []ads.Comment{}
	for _, v := range f.operations.Comments {
		if (adID == "" || v.AdID == adID) && v.Status != "DELETED" {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (f *Backend) GetBillingBalance(ctx context.Context) (ads.BillingBalance, error) {
	if err := ctx.Err(); err != nil {
		return ads.BillingBalance{}, err
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.operations.Balance, nil
}
func (f *Backend) ListBillingTransactions(ctx context.Context, start, end string) ([]ads.BillingTransaction, error) {
	if err := ads.ValidateDateRange(start, end, 93); err != nil {
		return nil, err
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := []ads.BillingTransaction{}
	for _, v := range f.operations.Transactions {
		day := v.OccurredAt.In(f.location).Format(time.DateOnly)
		if day >= start && day <= end {
			out = append(out, v)
		}
	}
	return out, nil
}

func (f *Backend) comment(id string) (ads.Comment, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	v, ok := f.operations.Comments[id]
	if !ok {
		return ads.Comment{}, ads.ErrNotFound
	}
	return v, nil
}
func mustSources(f *Backend, ctx context.Context) []ads.EventSource {
	v, _ := f.ListEventSources(ctx)
	return v
}
func mustIdentities(f *Backend, ctx context.Context) []ads.Identity {
	v, _ := f.ListIdentities(ctx)
	return v
}
func mergeAdGroup(dst *ads.AdGroupUpdateSpec, src ads.AdGroupUpdateSpec) {
	dst.AdGroupID = src.AdGroupID
	if src.Budget != nil {
		dst.Budget = src.Budget
	}
	if src.Bid != nil {
		dst.Bid = src.Bid
	}
	if src.ScheduleEnd != "" {
		dst.ScheduleEnd = src.ScheduleEnd
	}
	if src.ScheduleStart != "" {
		dst.ScheduleStart = src.ScheduleStart
	}
	if len(src.Placements) > 0 {
		dst.Placements = src.Placements
	}
	if len(src.AudienceIDs) > 0 {
		dst.AudienceIDs = src.AudienceIDs
	}
	if len(src.ExcludedAudienceIDs) > 0 {
		dst.ExcludedAudienceIDs = src.ExcludedAudienceIDs
	}
	if len(src.LocationIDs) > 0 {
		dst.LocationIDs = src.LocationIDs
	}
	if len(src.Languages) > 0 {
		dst.Languages = src.Languages
	}
}
func mergeAdCreative(dst *ads.AdCreativeUpdateSpec, src ads.AdCreativeUpdateSpec) {
	dst.AdID = src.AdID
	if src.IdentityID != "" {
		dst.IdentityID = src.IdentityID
		dst.IdentityType = src.IdentityType
	}
	if src.AssetID != "" {
		dst.AssetID = src.AssetID
		dst.AssetKind = src.AssetKind
	}
	if src.PrimaryText != "" {
		dst.PrimaryText = src.PrimaryText
	}
	if src.CallToAction != "" {
		dst.CallToAction = src.CallToAction
	}
	if src.DestinationURL != "" {
		dst.DestinationURL = src.DestinationURL
	}
}
func resource(v ads.Entity) ads.OperationResource {
	return ads.OperationResource{Kind: string(v.Level), ID: v.ID, Name: v.Name}
}
func rejected(err error) ads.OperationOutcome {
	return ads.OperationOutcome{State: "rejected", Message: err.Error()}
}
func partial(base ads.OperationOutcome, err error, created ads.Entity) ads.OperationOutcome {
	base.State = "partial"
	base.Message = err.Error()
	base.Resources = []ads.OperationResource{resource(created)}
	return base
}

var _ ads.Operations = (*Backend)(nil)
var _ ads.OperationsReader = (*Backend)(nil)
