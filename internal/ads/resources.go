package ads

import (
	"context"
	"errors"
	"time"
)

// The resource contracts below describe advertising concepts shared by the
// TikTok and local Sandbox backends. Provider-only request fields stay inside
// their adapters; lead response values and customer-list rows are never part
// of these model-visible contracts.

type Identity struct {
	ID        string `json:"id"`
	AccountID string `json:"account_id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Status    string `json:"status"`
}

type CreativeAsset struct {
	ID           string    `json:"id"`
	AccountID    string    `json:"account_id"`
	Name         string    `json:"name"`
	Kind         string    `json:"kind"` // image | video | playable | page
	Status       string    `json:"status"`
	ReviewStatus string    `json:"review_status"`
	Width        int       `json:"width,omitempty"`
	Height       int       `json:"height,omitempty"`
	DurationMS   int64     `json:"duration_ms,omitempty"`
	UpdatedAt    time.Time `json:"updated_at,omitempty"`
}

// CreativeMedia is optional operator-facing preview media. It is deliberately
// separate from CreativeAsset so backend resource reads and model tools do not
// inherit local media-hosting details.
type CreativeMedia struct {
	Kind        string `json:"kind"` // image | video
	PreviewURL  string `json:"preview_url"`
	PosterURL   string `json:"poster_url,omitempty"`
	Attribution string `json:"attribution,omitempty"`
	SourceURL   string `json:"source_url,omitempty"`
}

// AdDetail is an optional read model for the operator workspace. It connects one ad
// to the identity and asset actually returned by a backend without widening Entity,
// the cross-backend mutation contract.
type AdDetail struct {
	Ad             Entity         `json:"ad"`
	Identity       *Identity      `json:"identity,omitempty"`
	Creative       *CreativeAsset `json:"creative,omitempty"`
	PrimaryText    string         `json:"primary_text,omitempty"`
	CallToAction   string         `json:"call_to_action,omitempty"`
	DestinationURL string         `json:"destination_url,omitempty"`
	Format         string         `json:"format,omitempty"`
	Media          *CreativeMedia `json:"media,omitempty"`
	Limitations    []string       `json:"limitations,omitempty"`
}

type AdDetailsReader interface {
	GetAdDetail(context.Context, string) (AdDetail, error)
}

type Audience struct {
	ID              string    `json:"id"`
	AccountID       string    `json:"account_id"`
	Name            string    `json:"name"`
	Kind            string    `json:"kind"` // custom | lookalike | saved
	Status          string    `json:"status"`
	ApproximateSize *int64    `json:"approximate_size,omitempty"`
	Source          string    `json:"source,omitempty"`
	UpdatedAt       time.Time `json:"updated_at,omitempty"`
	PrivacyLimited  bool      `json:"privacy_limited"`
}

type AudienceOverlap struct {
	LeftID       string   `json:"left_id"`
	RightID      string   `json:"right_id"`
	OverlapUsers *int64   `json:"overlap_users,omitempty"`
	LeftRate     *float64 `json:"left_rate,omitempty"`
	RightRate    *float64 `json:"right_rate,omitempty"`
	Complete     bool     `json:"complete"`
	Limitations  []string `json:"limitations,omitempty"`
}

type TargetingOption struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	ParentID string `json:"parent_id,omitempty"`
	Enabled  bool   `json:"enabled"`
}

type EventSource struct {
	ID          string     `json:"id"`
	AccountID   string     `json:"account_id"`
	Name        string     `json:"name"`
	Kind        string     `json:"kind"` // pixel | app | offline | crm
	Status      string     `json:"status"`
	LastEventAt *time.Time `json:"last_event_at,omitempty"`
	EventTypes  []string   `json:"event_types,omitempty"`
}

type EventStats struct {
	SourceID    string           `json:"source_id"`
	Start       string           `json:"start_date"`
	End         string           `json:"end_date"`
	Events      map[string]int64 `json:"events"`
	Complete    bool             `json:"complete"`
	Limitations []string         `json:"limitations,omitempty"`
}

type AttributionSettings struct {
	ClickWindowDays int      `json:"click_window_days"`
	ViewWindowDays  int      `json:"view_window_days"`
	Basis           string   `json:"basis"`
	Limitations     []string `json:"limitations,omitempty"`
}

type LeadForm struct {
	ID         string    `json:"id"`
	AccountID  string    `json:"account_id"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	FieldNames []string  `json:"field_names,omitempty"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

type Catalog struct {
	ID           string `json:"id"`
	AccountID    string `json:"account_id"`
	Name         string `json:"name"`
	Currency     string `json:"currency"`
	Status       string `json:"status"`
	ProductCount int64  `json:"product_count"`
	IssueCount   int64  `json:"issue_count"`
}

type ProductSet struct {
	ID           string `json:"id"`
	CatalogID    string `json:"catalog_id"`
	Name         string `json:"name"`
	ProductCount int64  `json:"product_count"`
}

type AutomatedRule struct {
	ID          string `json:"id"`
	AccountID   string `json:"account_id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	TargetLevel Level  `json:"target_level"`
	Action      string `json:"action"`
	Schedule    string `json:"schedule"`
}

type AutomatedRuleResult struct {
	ID            string    `json:"id"`
	RuleID        string    `json:"rule_id"`
	ExecutedAt    time.Time `json:"executed_at"`
	Status        string    `json:"status"`
	AffectedCount int64     `json:"affected_count"`
}

type CommonAdsReader interface {
	ListIdentities(context.Context) ([]Identity, error)
	ListCreativeAssets(context.Context) ([]CreativeAsset, error)
	GetCreativeAsset(context.Context, string) (CreativeAsset, error)
	ListAudiences(context.Context) ([]Audience, error)
	GetAudience(context.Context, string) (Audience, error)
	GetAudienceOverlap(context.Context, string, string) (AudienceOverlap, error)
	ListTargetingOptions(context.Context, string) ([]TargetingOption, error)
	ListEventSources(context.Context) ([]EventSource, error)
	GetEventStats(context.Context, string, string, string) (EventStats, error)
	GetAttributionSettings(context.Context) (AttributionSettings, error)
	ListLeadForms(context.Context) ([]LeadForm, error)
	GetLeadForm(context.Context, string) (LeadForm, error)
	ListCatalogs(context.Context) ([]Catalog, error)
	ListProductSets(context.Context, string) ([]ProductSet, error)
	ListAutomatedRules(context.Context) ([]AutomatedRule, error)
	ListAutomatedRuleResults(context.Context, string) ([]AutomatedRuleResult, error)
}

func ValidateDateRange(start, end string, maxDays int) error {
	a, err := time.Parse(time.DateOnly, start)
	if err != nil {
		return errors.New("invalid start_date")
	}
	b, err := time.Parse(time.DateOnly, end)
	if err != nil || b.Before(a) || b.Sub(a) > time.Duration(maxDays-1)*24*time.Hour {
		return errors.New("invalid date range")
	}
	return nil
}
