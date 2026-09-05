package app

import (
	"context"

	"github.com/z-chenhao/ad-agent/internal/ads"
)

func (p persistentSandbox) common(ctx context.Context) (ads.CommonAdsReader, error) {
	b, err := p.load(ctx)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (p persistentSandbox) ListIdentities(ctx context.Context) ([]ads.Identity, error) {
	b, err := p.common(ctx)
	if err != nil {
		return nil, err
	}
	return b.ListIdentities(ctx)
}
func (p persistentSandbox) ListCreativeAssets(ctx context.Context) ([]ads.CreativeAsset, error) {
	b, err := p.common(ctx)
	if err != nil {
		return nil, err
	}
	return b.ListCreativeAssets(ctx)
}
func (p persistentSandbox) GetCreativeAsset(ctx context.Context, id string) (ads.CreativeAsset, error) {
	b, err := p.common(ctx)
	if err != nil {
		return ads.CreativeAsset{}, err
	}
	return b.GetCreativeAsset(ctx, id)
}

func (p persistentSandbox) GetAdDetail(ctx context.Context, id string) (ads.AdDetail, error) {
	b, err := p.load(ctx)
	if err != nil {
		return ads.AdDetail{}, err
	}
	return b.GetAdDetail(ctx, id)
}
func (p persistentSandbox) ListAudiences(ctx context.Context) ([]ads.Audience, error) {
	b, err := p.common(ctx)
	if err != nil {
		return nil, err
	}
	return b.ListAudiences(ctx)
}
func (p persistentSandbox) GetAudience(ctx context.Context, id string) (ads.Audience, error) {
	b, err := p.common(ctx)
	if err != nil {
		return ads.Audience{}, err
	}
	return b.GetAudience(ctx, id)
}
func (p persistentSandbox) GetAudienceOverlap(ctx context.Context, left, right string) (ads.AudienceOverlap, error) {
	b, err := p.common(ctx)
	if err != nil {
		return ads.AudienceOverlap{}, err
	}
	return b.GetAudienceOverlap(ctx, left, right)
}
func (p persistentSandbox) ListTargetingOptions(ctx context.Context, kind string) ([]ads.TargetingOption, error) {
	b, err := p.common(ctx)
	if err != nil {
		return nil, err
	}
	return b.ListTargetingOptions(ctx, kind)
}
func (p persistentSandbox) ListEventSources(ctx context.Context) ([]ads.EventSource, error) {
	b, err := p.common(ctx)
	if err != nil {
		return nil, err
	}
	return b.ListEventSources(ctx)
}
func (p persistentSandbox) GetEventStats(ctx context.Context, id, start, end string) (ads.EventStats, error) {
	b, err := p.common(ctx)
	if err != nil {
		return ads.EventStats{}, err
	}
	return b.GetEventStats(ctx, id, start, end)
}
func (p persistentSandbox) GetAttributionSettings(ctx context.Context) (ads.AttributionSettings, error) {
	b, err := p.common(ctx)
	if err != nil {
		return ads.AttributionSettings{}, err
	}
	return b.GetAttributionSettings(ctx)
}
func (p persistentSandbox) ListLeadForms(ctx context.Context) ([]ads.LeadForm, error) {
	b, err := p.common(ctx)
	if err != nil {
		return nil, err
	}
	return b.ListLeadForms(ctx)
}
func (p persistentSandbox) GetLeadForm(ctx context.Context, id string) (ads.LeadForm, error) {
	b, err := p.common(ctx)
	if err != nil {
		return ads.LeadForm{}, err
	}
	return b.GetLeadForm(ctx, id)
}
func (p persistentSandbox) ListCatalogs(ctx context.Context) ([]ads.Catalog, error) {
	b, err := p.common(ctx)
	if err != nil {
		return nil, err
	}
	return b.ListCatalogs(ctx)
}
func (p persistentSandbox) ListProductSets(ctx context.Context, catalogID string) ([]ads.ProductSet, error) {
	b, err := p.common(ctx)
	if err != nil {
		return nil, err
	}
	return b.ListProductSets(ctx, catalogID)
}
func (p persistentSandbox) ListAutomatedRules(ctx context.Context) ([]ads.AutomatedRule, error) {
	b, err := p.common(ctx)
	if err != nil {
		return nil, err
	}
	return b.ListAutomatedRules(ctx)
}
func (p persistentSandbox) ListAutomatedRuleResults(ctx context.Context, ruleID string) ([]ads.AutomatedRuleResult, error) {
	b, err := p.common(ctx)
	if err != nil {
		return nil, err
	}
	return b.ListAutomatedRuleResults(ctx, ruleID)
}

func (p persistentSandbox) ListComments(ctx context.Context, adID string, limit int) ([]ads.Comment, error) {
	b, err := p.load(ctx)
	if err != nil {
		return nil, err
	}
	return b.ListComments(ctx, adID, limit)
}
func (p persistentSandbox) GetBillingBalance(ctx context.Context) (ads.BillingBalance, error) {
	b, err := p.load(ctx)
	if err != nil {
		return ads.BillingBalance{}, err
	}
	return b.GetBillingBalance(ctx)
}
func (p persistentSandbox) ListBillingTransactions(ctx context.Context, start, end string) ([]ads.BillingTransaction, error) {
	b, err := p.load(ctx)
	if err != nil {
		return nil, err
	}
	return b.ListBillingTransactions(ctx, start, end)
}

var _ ads.CommonAdsReader = persistentSandbox{}
var _ ads.AdDetailsReader = persistentSandbox{}
var _ ads.OperationsReader = persistentSandbox{}
var _ ads.Operations = persistentSandbox{}
