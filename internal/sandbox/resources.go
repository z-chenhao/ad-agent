package sandbox

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/z-chenhao/ad-agent/internal/ads"
)

func (f *Backend) ListIdentities(ctx context.Context) ([]ads.Identity, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []ads.Identity{
		{ID: "identity_aster_pine", AccountID: f.account.ID, Name: "Aster & Pine", Kind: "brand", Status: "ACTIVE"},
		{ID: "identity_maya_home", AccountID: f.account.ID, Name: "Maya Makes Space", Kind: "creator", Status: "AUTHORIZED"},
	}, nil
}

func (f *Backend) ListCreativeAssets(ctx context.Context) ([]ads.CreativeAsset, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	epoch := f.simulationStart.Add(-6 * time.Hour)
	return []ads.CreativeAsset{
		{ID: "creative_creator_pov", AccountID: f.account.ID, Name: "Vintage shelf styling · room tour", Kind: "video", Status: "READY", ReviewStatus: "APPROVED", Width: 540, Height: 960, DurationMS: 13013, UpdatedAt: epoch},
		{ID: "creative_modular_demo", AccountID: f.account.ID, Name: "Warm loft · room tour", Kind: "video", Status: "READY", ReviewStatus: "APPROVED", Width: 1280, Height: 720, DurationMS: 10027, UpdatedAt: epoch.Add(-72 * time.Hour)},
		{ID: "creative_room_reveal_v1", AccountID: f.account.ID, Name: "Warm loft · styled room", Kind: "image", Status: "READY", ReviewStatus: "APPROVED", Width: 900, Height: 506, UpdatedAt: epoch.Add(-9 * 24 * time.Hour)},
		{ID: "creative_entryway", AccountID: f.account.ID, Name: "Sunlit frames · wall-art edit", Kind: "image", Status: "READY", ReviewStatus: "APPROVED", Width: 900, Height: 1600, UpdatedAt: epoch.Add(-36 * time.Hour)},
		{ID: "creative_unboxing", AccountID: f.account.ID, Name: "Vintage shelves · collected objects", Kind: "image", Status: "READY", ReviewStatus: "APPROVED", Width: 900, Height: 1600, UpdatedAt: epoch.Add(-4 * 24 * time.Hour)},
		{ID: "creative_customer_review_v1", AccountID: f.account.ID, Name: "Home office · workspace edit", Kind: "image", Status: "READY", ReviewStatus: "APPROVED", Width: 900, Height: 1600, UpdatedAt: epoch.Add(-15 * 24 * time.Hour)},
		{ID: "creative_founder_materials", AccountID: f.account.ID, Name: "Bright workspace · design edit", Kind: "image", Status: "READY", ReviewStatus: "APPROVED", Width: 900, Height: 506, UpdatedAt: epoch.Add(-7 * 24 * time.Hour)},
		{ID: "creative_shipping_offer", AccountID: f.account.ID, Name: "Modern living · room edit", Kind: "image", Status: "READY", ReviewStatus: "APPROVED", Width: 900, Height: 1600, UpdatedAt: epoch.Add(-3 * 24 * time.Hour)},
		{ID: "creative_social_proof", AccountID: f.account.ID, Name: "Warm loft · revisit the collection", Kind: "image", Status: "READY", ReviewStatus: "APPROVED", Width: 900, Height: 506, UpdatedAt: epoch.Add(-28 * time.Hour)},
		{ID: "creative_stock_message_v1", AccountID: f.account.ID, Name: "Sunlit frames · saved-cart reminder", Kind: "image", Status: "READY", ReviewStatus: "LIMITED", Width: 900, Height: 1600, UpdatedAt: epoch.Add(-5 * 24 * time.Hour)},
		{ID: "creative_fall_teaser_v1", AccountID: f.account.ID, Name: "Home office · fall preview", Kind: "image", Status: "READY", ReviewStatus: "APPROVED", Width: 900, Height: 1600, UpdatedAt: epoch.Add(-12 * 24 * time.Hour)},
		{ID: "creative_fall_montage", AccountID: f.account.ID, Name: "Modern living · fall preview", Kind: "image", Status: "READY", ReviewStatus: "APPROVED", Width: 900, Height: 1600, UpdatedAt: epoch.Add(-10 * 24 * time.Hour)},
	}, nil
}

func (f *Backend) GetAdDetail(ctx context.Context, id string) (ads.AdDetail, error) {
	ad, err := f.Get(ctx, ads.Ad, id)
	if err != nil {
		return ads.AdDetail{}, err
	}
	creativeIDs := map[string]string{
		"ad_prospect_creator": "creative_creator_pov", "ad_prospect_demo": "creative_modular_demo",
		"ad_interest_room": "creative_room_reveal_v1", "ad_interest_before": "creative_entryway",
		"ad_lal_unboxing": "creative_unboxing", "ad_lal_review": "creative_customer_review_v1",
		"ad_viewers_founder": "creative_founder_materials", "ad_viewers_offer": "creative_shipping_offer",
		"ad_cart_proof": "creative_social_proof", "ad_cart_urgency": "creative_stock_message_v1",
		"ad_launch_teaser": "creative_fall_teaser_v1", "ad_launch_collection": "creative_fall_montage",
	}
	media := map[string]ads.CreativeMedia{
		"warm_loft_video": {
			Kind: "video", PreviewURL: "/sandbox/creatives/warm-loft.mp4", PosterURL: "/sandbox/creatives/warm-loft.jpg",
			Attribution: "Video by Charlotte May on Pexels", SourceURL: "https://www.pexels.com/video/home-interior-decor-5823846/",
		},
		"vintage_shelves_video": {
			Kind: "video", PreviewURL: "/sandbox/creatives/vintage-shelves.mp4", PosterURL: "/sandbox/creatives/vintage-shelves.jpg",
			Attribution: "Video by Charlotte May on Pexels", SourceURL: "https://www.pexels.com/video/house-interior-design-5823678/",
		},
		"warm_loft": {
			Kind: "image", PreviewURL: "/sandbox/creatives/warm-loft.jpg",
			Attribution: "Video still by Charlotte May on Pexels", SourceURL: "https://www.pexels.com/video/home-interior-decor-5823846/",
		},
		"vintage_shelves": {
			Kind: "image", PreviewURL: "/sandbox/creatives/vintage-shelves.jpg",
			Attribution: "Video still by Charlotte May on Pexels", SourceURL: "https://www.pexels.com/video/house-interior-design-5823678/",
		},
		"sunlit_frames": {
			Kind: "image", PreviewURL: "/sandbox/creatives/sunlit-frames.jpg",
			Attribution: "Video still by Angela Roma on Pexels", SourceURL: "https://www.pexels.com/video/blank-frames-leaning-the-wall-7316027/",
		},
		"home_office": {
			Kind: "image", PreviewURL: "/sandbox/creatives/home-office.jpg",
			Attribution: "Video still by Charlotte May on Pexels", SourceURL: "https://www.pexels.com/video/a-footage-of-a-home-office-5823581/",
		},
		"bright_office": {
			Kind: "image", PreviewURL: "/sandbox/creatives/bright-office.jpg",
			Attribution: "Video still by Curtis Adams on Pexels", SourceURL: "https://www.pexels.com/video/elegant-furniture-in-room-11593542/",
		},
		"modern_living_room": {
			Kind: "image", PreviewURL: "/sandbox/creatives/modern-living-room.jpg",
			Attribution: "Video still by Ali Jafar on Pexels", SourceURL: "https://www.pexels.com/video/modern-living-room-interior-design-36842451/",
		},
	}
	mediaIDs := map[string]string{
		"ad_prospect_creator": "vintage_shelves_video", "ad_prospect_demo": "warm_loft_video",
		"ad_interest_room": "warm_loft", "ad_interest_before": "sunlit_frames",
		"ad_lal_unboxing": "vintage_shelves", "ad_lal_review": "home_office",
		"ad_viewers_founder": "bright_office", "ad_viewers_offer": "modern_living_room",
		"ad_cart_proof": "warm_loft", "ad_cart_urgency": "sunlit_frames",
		"ad_launch_teaser": "home_office", "ad_launch_collection": "modern_living_room",
	}
	texts := map[string]string{
		"ad_prospect_creator":  "Collected books, warm wood, and objects that make a corner yours. Explore the Aster & Pine decor edit.",
		"ad_prospect_demo":     "Tour a warm, lived-in loft. Discover furniture and accents inspired by the look.",
		"ad_interest_room":     "Warm wood and layered decor for a calmer room. Browse the loft collection.",
		"ad_interest_before":   "Let the light in. Explore simple frames and wall-art pairings for your space.",
		"ad_lal_unboxing":      "A collected shelf starts with a few meaningful pieces. Explore our vintage-inspired decor edit.",
		"ad_lal_review":        "Make space for your working day. Explore our home-office collection.",
		"ad_viewers_founder":   "A brighter place to focus. Explore desks, seating, and home-office accents.",
		"ad_viewers_offer":     "Still thinking about a room refresh? Revisit the modern-living collection.",
		"ad_cart_proof":        "Bring the warm-loft look home. Revisit the furniture and accents you explored.",
		"ad_cart_urgency":      "Your wall-art edit is waiting. Return to your saved selection when you are ready.",
		"ad_launch_teaser":     "A new season at home. Preview the fall workspace edit.",
		"ad_launch_collection": "A considered room for everyday living. Preview the fall home edit.",
	}
	// Reusing an installed asset keeps its actual preview, even on a newly created
	// ad. Unknown uploaded assets never borrow unrelated stock imagery.
	assetPreview := func(assetID string) *ads.CreativeMedia {
		for adID, creativeID := range creativeIDs {
			if creativeID == assetID {
				value := media[mediaIDs[adID]]
				return &value
			}
		}
		return nil
	}
	f.mu.RLock()
	override, overridden := f.operations.Ads[id]
	f.mu.RUnlock()
	if overridden && creativeIDs[id] == "" {
		creative, creativeErr := f.GetCreativeAsset(ctx, override.AssetID)
		if creativeErr != nil {
			return ads.AdDetail{}, creativeErr
		}
		identities, _ := f.ListIdentities(ctx)
		var identity *ads.Identity
		for i := range identities {
			if identities[i].ID == override.IdentityID {
				value := identities[i]
				identity = &value
				break
			}
		}
		return ads.AdDetail{Ad: ad, Identity: identity, Creative: &creative, PrimaryText: override.PrimaryText, CallToAction: override.CallToAction, DestinationURL: override.DestinationURL, Format: "SINGLE_" + strings.ToUpper(override.AssetKind), Media: assetPreview(creative.ID), Limitations: []string{"Platform review has not been performed."}}, nil
	}
	creative, err := f.GetCreativeAsset(ctx, creativeIDs[id])
	if err != nil {
		return ads.AdDetail{}, err
	}
	identities, _ := f.ListIdentities(ctx)
	identity := identities[0]
	if id == "ad_prospect_creator" || id == "ad_lal_unboxing" {
		identity = identities[1]
	}
	preview, ok := media[mediaIDs[id]]
	if !ok {
		return ads.AdDetail{}, errors.New("sandbox ad is missing preview media")
	}
	detail := ads.AdDetail{Ad: ad, Identity: &identity, Creative: &creative, PrimaryText: texts[id], CallToAction: "SHOP_NOW", DestinationURL: "https://asterandpine.test/collections/home-edit", Format: "SINGLE_" + strings.ToUpper(preview.Kind), Media: &preview, Limitations: []string{}}
	if overridden {
		if override.AssetID != "" {
			value, assetErr := f.GetCreativeAsset(ctx, override.AssetID)
			if assetErr != nil {
				return ads.AdDetail{}, assetErr
			}
			detail.Creative, detail.Media = &value, assetPreview(value.ID)
			detail.Format = "SINGLE_" + strings.ToUpper(value.Kind)
		}
		if override.IdentityID != "" {
			for _, value := range identities {
				if value.ID == override.IdentityID {
					item := value
					detail.Identity = &item
				}
			}
		}
		if override.PrimaryText != "" {
			detail.PrimaryText = override.PrimaryText
		}
		if override.CallToAction != "" {
			detail.CallToAction = override.CallToAction
		}
		if override.DestinationURL != "" {
			detail.DestinationURL = override.DestinationURL
		}
	}
	return detail, nil
}

func (f *Backend) GetCreativeAsset(ctx context.Context, id string) (ads.CreativeAsset, error) {
	values, err := f.ListCreativeAssets(ctx)
	if err != nil {
		return ads.CreativeAsset{}, err
	}
	for _, value := range values {
		if value.ID == id {
			return value, nil
		}
	}
	return ads.CreativeAsset{}, ads.ErrNotFound
}

func (f *Backend) ListAudiences(ctx context.Context) ([]ads.Audience, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	prospects, purchasers, lookalike := int64(420000), int64(18500), int64(780000)
	values := []ads.Audience{
		{ID: "audience_prospecting", AccountID: f.account.ID, Name: "Broad prospecting | US | 18-54", Kind: "saved", Status: "READY", ApproximateSize: &prospects, Source: "targeting", UpdatedAt: f.clock, PrivacyLimited: false},
		{ID: "audience_purchasers", AccountID: f.account.ID, Name: "Purchasers 180D", Kind: "custom", Status: "READY", ApproximateSize: &purchasers, Source: "pixel_aster_web", UpdatedAt: f.clock.Add(-time.Hour), PrivacyLimited: true},
		{ID: "audience_lookalike", AccountID: f.account.ID, Name: "Purchaser lookalike 3% | US", Kind: "lookalike", Status: "READY", ApproximateSize: &lookalike, Source: "audience_purchasers", UpdatedAt: f.clock.Add(-3 * time.Hour), PrivacyLimited: true},
	}
	f.mu.RLock()
	for _, value := range f.operations.Audiences {
		values = append(values, value)
	}
	f.mu.RUnlock()
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	return values, nil
}

func (f *Backend) GetAudience(ctx context.Context, id string) (ads.Audience, error) {
	values, err := f.ListAudiences(ctx)
	if err != nil {
		return ads.Audience{}, err
	}
	for _, value := range values {
		if value.ID == id {
			return value, nil
		}
	}
	return ads.Audience{}, ads.ErrNotFound
}

func (f *Backend) GetAudienceOverlap(ctx context.Context, left, right string) (ads.AudienceOverlap, error) {
	if left == right {
		return ads.AudienceOverlap{}, errors.New("audiences must be distinct")
	}
	leftAudience, err := f.GetAudience(ctx, left)
	if err != nil {
		return ads.AudienceOverlap{}, err
	}
	rightAudience, err := f.GetAudience(ctx, right)
	if err != nil {
		return ads.AudienceOverlap{}, err
	}
	if leftAudience.ApproximateSize == nil || rightAudience.ApproximateSize == nil {
		return ads.AudienceOverlap{LeftID: left, RightID: right, Complete: false, Limitations: []string{"Sandbox audience size is unavailable."}}, nil
	}
	overlap := *leftAudience.ApproximateSize / 10
	if *rightAudience.ApproximateSize < *leftAudience.ApproximateSize {
		overlap = *rightAudience.ApproximateSize / 10
	}
	leftRate := float64(overlap) / float64(*leftAudience.ApproximateSize)
	rightRate := float64(overlap) / float64(*rightAudience.ApproximateSize)
	return ads.AudienceOverlap{LeftID: left, RightID: right, OverlapUsers: &overlap, LeftRate: &leftRate, RightRate: &rightRate, Complete: true}, nil
}

func (f *Backend) ListTargetingOptions(ctx context.Context, kind string) ([]ads.TargetingOption, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	all := []ads.TargetingOption{
		{ID: "US", Kind: "location", Name: "United States", Enabled: true},
		{ID: "interest_fitness", Kind: "interest", Name: "Fitness and wellness", Enabled: true},
		{ID: "interest_home", Kind: "interest", Name: "Home and garden", Enabled: true},
		{ID: "device_ios", Kind: "device", Name: "iOS", Enabled: true},
		{ID: "device_android", Kind: "device", Name: "Android", Enabled: true},
		{ID: "en", Kind: "language", Name: "English", Enabled: true},
		{ID: "PLACEMENT_TIKTOK", Kind: "placement", Name: "TikTok", Enabled: true},
	}
	out := make([]ads.TargetingOption, 0, len(all))
	for _, value := range all {
		if kind == "" || value.Kind == kind {
			out = append(out, value)
		}
	}
	if kind != "" && len(out) == 0 {
		return nil, errors.New("unsupported targeting option kind")
	}
	return out, nil
}

func (f *Backend) ListEventSources(ctx context.Context) ([]ads.EventSource, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	latest := f.clock.Add(-35 * time.Minute)
	values := []ads.EventSource{
		{ID: "pixel_aster_web", AccountID: f.account.ID, Name: "Aster & Pine web checkout", Kind: "pixel", Status: "ACTIVE", LastEventAt: &latest, EventTypes: []string{"ViewContent", "AddToCart", "InitiateCheckout", "Purchase"}},
		{ID: "offline_showroom", AccountID: f.account.ID, Name: "Brooklyn showroom purchases", Kind: "offline", Status: "ACTIVE", LastEventAt: &latest, EventTypes: []string{"Purchase"}},
	}
	f.mu.RLock()
	for _, value := range f.operations.EventSources {
		values = append(values, value)
	}
	f.mu.RUnlock()
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	return values, nil
}

func (f *Backend) GetEventStats(ctx context.Context, sourceID, start, end string) (ads.EventStats, error) {
	if err := ads.ValidateDateRange(start, end, 31); err != nil {
		return ads.EventStats{}, err
	}
	sources, err := f.ListEventSources(ctx)
	if err != nil {
		return ads.EventStats{}, err
	}
	var source *ads.EventSource
	for i := range sources {
		if sources[i].ID == sourceID {
			source = &sources[i]
		}
	}
	if source == nil {
		return ads.EventStats{}, ads.ErrNotFound
	}
	// Source creation alone does not manufacture telemetry. Only linked ad groups
	// contribute observed purchases; other funnel events are not modeled yet.
	f.mu.RLock()
	groups := []string{}
	for id, entity := range f.entities {
		if entity.Level != ads.AdGroup {
			continue
		}
		pixel := f.operations.AdGroupDefinitions[id].PixelID
		if pixel == "" && sourceID == "pixel_aster_web" {
			if _, created := f.operations.AdGroupDefinitions[id]; !created {
				pixel = sourceID
			}
		}
		if pixel == sourceID {
			groups = append(groups, id)
		}
	}
	f.mu.RUnlock()
	events := map[string]int64{"Purchase": 0}
	for _, id := range groups {
		report, err := f.Report(ctx, ads.ReportQuery{Level: ads.AdGroup, EntityID: id, Start: start, End: end})
		if err != nil {
			return ads.EventStats{}, err
		}
		if report.Totals.Conversions != nil {
			events["Purchase"] += report.Totals.Conversions.IntPart()
		}
	}
	return ads.EventStats{SourceID: sourceID, Start: start, End: end, Events: events, Complete: false, Limitations: []string{"Purchase counts reflect reported attributed conversions from linked ad groups, not all site events. Unlinked sources have no events. ViewContent, cart, offline ingestion, and un-attributed purchases are not modeled."}}, nil
}

func (f *Backend) GetAttributionSettings(ctx context.Context) (ads.AttributionSettings, error) {
	if err := ctx.Err(); err != nil {
		return ads.AttributionSettings{}, err
	}
	return ads.AttributionSettings{ClickWindowDays: 7, ViewWindowDays: 1, Basis: "sandbox last-touch", Limitations: []string{}}, nil
}

func (f *Backend) ListLeadForms(ctx context.Context) ([]ads.LeadForm, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []ads.LeadForm{
		{ID: "lead_form_trade", AccountID: f.account.ID, Name: "Trade program application", Status: "ACTIVE", FieldNames: []string{"email", "full_name", "company"}, UpdatedAt: f.clock.Add(-5 * 24 * time.Hour)},
		{ID: "lead_form_catalog", AccountID: f.account.ID, Name: "Fall lookbook", Status: "PAUSED", FieldNames: []string{"email"}, UpdatedAt: f.clock.Add(-24 * time.Hour)},
	}, nil
}

func (f *Backend) GetLeadForm(ctx context.Context, id string) (ads.LeadForm, error) {
	forms, err := f.ListLeadForms(ctx)
	if err != nil {
		return ads.LeadForm{}, err
	}
	for _, value := range forms {
		if value.ID == id {
			return value, nil
		}
	}
	return ads.LeadForm{}, ads.ErrNotFound
}

func (f *Backend) ListCatalogs(ctx context.Context) ([]ads.Catalog, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []ads.Catalog{{ID: "catalog_home_us", AccountID: f.account.ID, Name: "US home collection", Currency: f.account.Currency, Status: "ACTIVE", ProductCount: 184, IssueCount: 3}}, nil
}

func (f *Backend) ListProductSets(ctx context.Context, catalogID string) ([]ads.ProductSet, error) {
	catalogs, err := f.ListCatalogs(ctx)
	if err != nil {
		return nil, err
	}
	found := false
	for _, catalog := range catalogs {
		found = found || catalog.ID == catalogID
	}
	if !found {
		return nil, ads.ErrNotFound
	}
	return []ads.ProductSet{
		{ID: "product_set_bestsellers", CatalogID: catalogID, Name: "Bestsellers", ProductCount: 24},
		{ID: "product_set_new", CatalogID: catalogID, Name: "New arrivals", ProductCount: 18},
	}, nil
}

func (f *Backend) ListAutomatedRules(ctx context.Context) ([]ads.AutomatedRule, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	values := []ads.AutomatedRule{
		{ID: "rule_low_spend", AccountID: f.account.ID, Name: "Pause low efficiency ad groups", Status: "DISABLED", TargetLevel: ads.AdGroup, Action: "PAUSE", Schedule: "HOURLY"},
		{ID: "rule_budget_guard", AccountID: f.account.ID, Name: "Notify on daily budget pace", Status: "DISABLED", TargetLevel: ads.Campaign, Action: "NOTIFY", Schedule: "HOURLY"},
	}
	f.mu.RLock()
	for _, value := range f.operations.Rules {
		values = append(values, value)
	}
	f.mu.RUnlock()
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	return values, nil
}

func (f *Backend) ListAutomatedRuleResults(ctx context.Context, ruleID string) ([]ads.AutomatedRuleResult, error) {
	rules, err := f.ListAutomatedRules(ctx)
	if err != nil {
		return nil, err
	}
	found := false
	for _, rule := range rules {
		found = found || rule.ID == ruleID
	}
	if !found {
		return nil, ads.ErrNotFound
	}
	f.mu.RLock()
	values := []ads.AutomatedRuleResult{}
	for _, result := range f.operations.RuleResults {
		if result.RuleID == ruleID {
			values = append(values, result)
		}
	}
	f.mu.RUnlock()
	sort.Slice(values, func(i, j int) bool { return values[i].ExecutedAt.After(values[j].ExecutedAt) })
	return values, nil
}

var _ ads.CommonAdsReader = (*Backend)(nil)
var _ ads.AdDetailsReader = (*Backend)(nil)
