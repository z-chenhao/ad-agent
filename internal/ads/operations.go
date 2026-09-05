package ads

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// OperationKind is the bounded daily-operations vocabulary shared by the
// Sandbox and TikTok adapters. Provider request bodies stay private.
type OperationKind string

const (
	CreateCampaignBundle OperationKind = "campaign_bundle_create"
	UpdateAdGroup        OperationKind = "ad_group_update"
	UpdateAdCreative     OperationKind = "ad_creative_update"
	CreateAudience       OperationKind = "audience_create"
	CreateAutomatedRule  OperationKind = "automated_rule_create"
	ModerateComment      OperationKind = "comment_action"
	CreateEventSource    OperationKind = "event_source_create"
)

type CampaignSpec struct {
	Name       string           `json:"name"`
	Objective  string           `json:"objective"`
	BudgetMode string           `json:"budget_mode,omitempty"`
	Budget     *decimal.Decimal `json:"budget,omitempty"`
	Status     string           `json:"status"`
}

type AdGroupSpec struct {
	Name                string           `json:"name"`
	Budget              decimal.Decimal  `json:"budget"`
	BudgetMode          string           `json:"budget_mode"`
	BillingEvent        string           `json:"billing_event"`
	OptimizationGoal    string           `json:"optimization_goal"`
	OptimizationEvent   string           `json:"optimization_event,omitempty"`
	BidType             string           `json:"bid_type,omitempty"`
	Bid                 *decimal.Decimal `json:"bid,omitempty"`
	Pacing              string           `json:"pacing"`
	ScheduleType        string           `json:"schedule_type"`
	ScheduleStart       string           `json:"schedule_start"`
	ScheduleEnd         string           `json:"schedule_end,omitempty"`
	Placements          []string         `json:"placements"`
	LocationIDs         []string         `json:"location_ids"`
	Languages           []string         `json:"languages,omitempty"`
	AgeGroups           []string         `json:"age_groups,omitempty"`
	Gender              string           `json:"gender,omitempty"`
	AudienceIDs         []string         `json:"audience_ids,omitempty"`
	ExcludedAudienceIDs []string         `json:"excluded_audience_ids,omitempty"`
	PixelID             string           `json:"pixel_id,omitempty"`
	Status              string           `json:"status"`
}

type AdCreativeSpec struct {
	Name           string `json:"name"`
	IdentityID     string `json:"identity_id"`
	IdentityType   string `json:"identity_type"`
	AssetID        string `json:"asset_id"`
	AssetKind      string `json:"asset_kind"`
	PrimaryText    string `json:"primary_text"`
	CallToAction   string `json:"call_to_action"`
	DestinationURL string `json:"destination_url"`
	Status         string `json:"status"`
}

type CampaignBundleSpec struct {
	Campaign CampaignSpec     `json:"campaign"`
	AdGroup  AdGroupSpec      `json:"ad_group"`
	Ads      []AdCreativeSpec `json:"ads"`
}

type AdGroupUpdateSpec struct {
	AdGroupID           string           `json:"ad_group_id"`
	Budget              *decimal.Decimal `json:"budget,omitempty"`
	Bid                 *decimal.Decimal `json:"bid,omitempty"`
	ScheduleStart       string           `json:"schedule_start,omitempty"`
	ScheduleEnd         string           `json:"schedule_end,omitempty"`
	Placements          []string         `json:"placements,omitempty"`
	AudienceIDs         []string         `json:"audience_ids,omitempty"`
	ExcludedAudienceIDs []string         `json:"excluded_audience_ids,omitempty"`
	LocationIDs         []string         `json:"location_ids,omitempty"`
	Languages           []string         `json:"languages,omitempty"`
}

type AdCreativeUpdateSpec struct {
	AdID           string `json:"ad_id"`
	IdentityID     string `json:"identity_id,omitempty"`
	IdentityType   string `json:"identity_type,omitempty"`
	AssetID        string `json:"asset_id,omitempty"`
	AssetKind      string `json:"asset_kind,omitempty"`
	PrimaryText    string `json:"primary_text,omitempty"`
	CallToAction   string `json:"call_to_action,omitempty"`
	DestinationURL string `json:"destination_url,omitempty"`
}

type AudienceCreateSpec struct {
	Name             string   `json:"name"`
	Kind             string   `json:"kind"` // saved | lookalike
	SourceAudienceID string   `json:"source_audience_id,omitempty"`
	LocationIDs      []string `json:"location_ids,omitempty"`
	Languages        []string `json:"languages,omitempty"`
	AgeGroups        []string `json:"age_groups,omitempty"`
	Gender           string   `json:"gender,omitempty"`
	LookalikeRatio   int      `json:"lookalike_ratio,omitempty"`
}

type RuleCondition struct {
	Metric   string          `json:"metric"`
	Operator string          `json:"operator"`
	Value    decimal.Decimal `json:"value"`
	Window   string          `json:"window"`
}

type AutomatedRuleCreateSpec struct {
	Name        string           `json:"name"`
	TargetLevel Level            `json:"target_level"`
	TargetIDs   []string         `json:"target_ids"`
	Conditions  []RuleCondition  `json:"conditions"`
	Action      string           `json:"action"`
	ActionValue *decimal.Decimal `json:"action_value,omitempty"`
	Schedule    string           `json:"schedule"`
}

type CommentActionSpec struct {
	CommentID    string `json:"comment_id"`
	AdID         string `json:"ad_id"`
	TikTokItemID string `json:"tiktok_item_id"`
	Action       string `json:"action"` // reply | hide | unhide | delete
	Text         string `json:"text,omitempty"`
	IdentityID   string `json:"identity_id,omitempty"`
	IdentityType string `json:"identity_type,omitempty"`
}

type EventSourceCreateSpec struct {
	Name       string   `json:"name"`
	Kind       string   `json:"kind"` // pixel | offline
	EventTypes []string `json:"event_types,omitempty"`
}

type OperationRequest struct {
	Kind           OperationKind            `json:"kind"`
	CampaignBundle *CampaignBundleSpec      `json:"campaign_bundle,omitempty"`
	AdGroupUpdate  *AdGroupUpdateSpec       `json:"ad_group_update,omitempty"`
	AdUpdate       *AdCreativeUpdateSpec    `json:"ad_creative_update,omitempty"`
	Audience       *AudienceCreateSpec      `json:"audience,omitempty"`
	Rule           *AutomatedRuleCreateSpec `json:"automated_rule,omitempty"`
	Comment        *CommentActionSpec       `json:"comment,omitempty"`
	EventSource    *EventSourceCreateSpec   `json:"event_source,omitempty"`
}

type ChangeLine struct {
	Resource string `json:"resource"`
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	Field    string `json:"field"`
	Before   string `json:"before,omitempty"`
	After    string `json:"after"`
}

type OperationPlan struct {
	Request          OperationRequest `json:"request"`
	Lines            []ChangeLine     `json:"lines"`
	PreconditionHash string           `json:"precondition_hash"`
	SpendIncreasing  bool             `json:"spend_increasing"`
}

func (p OperationPlan) Version() string {
	b, _ := json.Marshal(p)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

type OperationResource struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type OperationOutcome struct {
	State      string              `json:"state"` // acknowledged | partial | rejected | unknown | not_sent
	RequestIDs []string            `json:"request_ids,omitempty"`
	Resources  []OperationResource `json:"resources,omitempty"`
	Message    string              `json:"message,omitempty"`
}

// OperationPlanner is the read-only half of the daily-operations contract. It
// may be given to the host even when platform writes are disabled.
type OperationPlanner interface {
	PrepareOperation(context.Context, OperationRequest) (OperationPlan, error)
}

// Operations is the approval-only half of the experimental typed extension.
// Apply is called only by the approval service and never retries.
type Operations interface {
	OperationPlanner
	ApplyOperation(context.Context, OperationPlan) OperationOutcome
	ReconcileOperation(context.Context, OperationPlan, OperationOutcome) (bool, error)
}

type Comment struct {
	ID           string    `json:"id"`
	AccountID    string    `json:"account_id"`
	AdID         string    `json:"ad_id"`
	TikTokItemID string    `json:"tiktok_item_id"`
	Author       string    `json:"author"`
	Text         string    `json:"text"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	ReplyCount   int64     `json:"reply_count"`
}

type BillingBalance struct {
	AccountID string          `json:"account_id"`
	Currency  string          `json:"currency"`
	Available decimal.Decimal `json:"available"`
	Cash      decimal.Decimal `json:"cash"`
	Voucher   decimal.Decimal `json:"voucher"`
	AsOf      time.Time       `json:"as_of"`
}

type BillingTransaction struct {
	ID         string          `json:"id"`
	AccountID  string          `json:"account_id"`
	Type       string          `json:"type"`
	Amount     decimal.Decimal `json:"amount"`
	Currency   string          `json:"currency"`
	OccurredAt time.Time       `json:"occurred_at"`
	Status     string          `json:"status"`
}

type OperationsReader interface {
	ListComments(context.Context, string, int) ([]Comment, error)
	GetBillingBalance(context.Context) (BillingBalance, error)
	ListBillingTransactions(context.Context, string, string) ([]BillingTransaction, error)
}

func (r OperationRequest) Validate() error {
	count := 0
	for _, present := range []bool{r.CampaignBundle != nil, r.AdGroupUpdate != nil, r.AdUpdate != nil, r.Audience != nil, r.Rule != nil, r.Comment != nil, r.EventSource != nil} {
		if present {
			count++
		}
	}
	if count != 1 {
		return errors.New("operation requires exactly one payload")
	}
	switch r.Kind {
	case CreateCampaignBundle:
		if r.CampaignBundle == nil {
			return errors.New("campaign bundle payload required")
		}
		return validateCampaignBundle(*r.CampaignBundle)
	case UpdateAdGroup:
		if r.AdGroupUpdate == nil || strings.TrimSpace(r.AdGroupUpdate.AdGroupID) == "" {
			return errors.New("ad group update payload required")
		}
		if r.AdGroupUpdate.Budget == nil && r.AdGroupUpdate.Bid == nil && r.AdGroupUpdate.ScheduleStart == "" && r.AdGroupUpdate.ScheduleEnd == "" && len(r.AdGroupUpdate.Placements)+len(r.AdGroupUpdate.AudienceIDs)+len(r.AdGroupUpdate.ExcludedAudienceIDs)+len(r.AdGroupUpdate.LocationIDs)+len(r.AdGroupUpdate.Languages) == 0 {
			return errors.New("ad group update is empty")
		}
		if r.AdGroupUpdate.Budget != nil && !r.AdGroupUpdate.Budget.IsPositive() || r.AdGroupUpdate.Bid != nil && !r.AdGroupUpdate.Bid.IsPositive() {
			return errors.New("budget and bid must be positive")
		}
		if overlaps(r.AdGroupUpdate.AudienceIDs, r.AdGroupUpdate.ExcludedAudienceIDs) {
			return errors.New("an audience cannot be both included and excluded")
		}
		if err := validateTimeOptional(r.AdGroupUpdate.ScheduleStart); err != nil {
			return err
		}
		if err := validateTimeOptional(r.AdGroupUpdate.ScheduleEnd); err != nil {
			return err
		}
		return validateScheduleOrder(r.AdGroupUpdate.ScheduleStart, r.AdGroupUpdate.ScheduleEnd)
	case UpdateAdCreative:
		if r.AdUpdate == nil || r.AdUpdate.AdID == "" {
			return errors.New("ad creative update payload required")
		}
		if r.AdUpdate.IdentityID == "" && r.AdUpdate.AssetID == "" && r.AdUpdate.PrimaryText == "" && r.AdUpdate.CallToAction == "" && r.AdUpdate.DestinationURL == "" {
			return errors.New("ad creative update is empty")
		}
		if r.AdUpdate.DestinationURL != "" && !validHTTPSURL(r.AdUpdate.DestinationURL) {
			return errors.New("invalid destination URL")
		}
		if len([]rune(r.AdUpdate.PrimaryText)) > 100 {
			return errors.New("ad primary text is too long")
		}
		return nil
	case CreateAudience:
		if r.Audience == nil || strings.TrimSpace(r.Audience.Name) == "" {
			return errors.New("audience payload required")
		}
		if r.Audience.Kind != "saved" && r.Audience.Kind != "lookalike" {
			return errors.New("unsupported audience kind")
		}
		if r.Audience.Kind == "saved" && len(r.Audience.LocationIDs) == 0 {
			return errors.New("saved audience requires locations")
		}
		if r.Audience.Kind == "lookalike" && (r.Audience.SourceAudienceID == "" || r.Audience.LookalikeRatio < 1 || r.Audience.LookalikeRatio > 10) {
			return errors.New("invalid lookalike audience")
		}
		return validateName(r.Audience.Name)
	case CreateAutomatedRule:
		if r.Rule == nil || strings.TrimSpace(r.Rule.Name) == "" || !r.Rule.TargetLevel.Valid() || r.Rule.TargetLevel == Advertiser || len(r.Rule.TargetIDs) == 0 || len(r.Rule.Conditions) == 0 {
			return errors.New("invalid automated rule")
		}
		if r.Rule.Action != "NOTIFY" && r.Rule.Action != "PAUSE" && r.Rule.Action != "CHANGE_BUDGET" {
			return errors.New("unsupported rule action")
		}
		if len(r.Rule.TargetIDs) > 50 || len(r.Rule.Conditions) > 8 || r.Rule.Schedule != "EVERY_30_MINUTES" {
			return errors.New("unsupported automated rule scope or schedule")
		}
		if r.Rule.Action == "CHANGE_BUDGET" && (r.Rule.ActionValue == nil || !r.Rule.ActionValue.IsPositive()) {
			return errors.New("rule action value required")
		}
		for _, condition := range r.Rule.Conditions {
			if !oneOf(condition.Metric, "SPEND", "CPA", "CTR", "CONVERSIONS") || !oneOf(condition.Operator, "GT", "LT") || !oneOf(condition.Window, "TODAY", "LAST_3_DAYS", "LAST_7_DAYS") || condition.Value.IsNegative() {
				return errors.New("unsupported automated rule condition")
			}
		}
		return validateName(r.Rule.Name)
	case ModerateComment:
		if r.Comment == nil || r.Comment.CommentID == "" || r.Comment.AdID == "" || r.Comment.TikTokItemID == "" {
			return errors.New("invalid comment action")
		}
		if r.Comment.Action != "reply" && r.Comment.Action != "hide" && r.Comment.Action != "unhide" && r.Comment.Action != "delete" {
			return errors.New("unsupported comment action")
		}
		if (r.Comment.Action == "reply" || r.Comment.Action == "delete") && (r.Comment.IdentityID == "" || r.Comment.IdentityType == "") {
			return errors.New("comment identity fields required")
		}
		if r.Comment.Action == "reply" && strings.TrimSpace(r.Comment.Text) == "" {
			return errors.New("reply fields required")
		}
		return nil
	case CreateEventSource:
		if r.EventSource == nil || strings.TrimSpace(r.EventSource.Name) == "" || (r.EventSource.Kind != "pixel" && r.EventSource.Kind != "offline") {
			return errors.New("invalid event source")
		}
		return validateName(r.EventSource.Name)
	default:
		return errors.New("unsupported operation")
	}
}

func validateCampaignBundle(v CampaignBundleSpec) error {
	if strings.TrimSpace(v.Campaign.Name) == "" || strings.TrimSpace(v.AdGroup.Name) == "" || len(v.Ads) == 0 || len(v.Ads) > 20 {
		return errors.New("invalid campaign bundle")
	}
	if v.Campaign.Objective != "TRAFFIC" && v.Campaign.Objective != "WEB_CONVERSIONS" {
		return errors.New("unsupported campaign objective")
	}
	if err := validateName(v.Campaign.Name); err != nil {
		return err
	}
	if err := validateName(v.AdGroup.Name); err != nil {
		return err
	}
	if overlaps(v.AdGroup.AudienceIDs, v.AdGroup.ExcludedAudienceIDs) {
		return errors.New("an audience cannot be both included and excluded")
	}
	if v.Campaign.Status != "DISABLE" || v.AdGroup.Status != "DISABLE" {
		return errors.New("new campaign bundle must start disabled")
	}
	if !v.AdGroup.Budget.IsPositive() || v.AdGroup.BudgetMode == "" || v.AdGroup.BillingEvent == "" || v.AdGroup.OptimizationGoal == "" || v.AdGroup.Pacing == "" || v.AdGroup.ScheduleType == "" || len(v.AdGroup.Placements) == 0 || len(v.AdGroup.LocationIDs) == 0 {
		return errors.New("campaign bundle is missing delivery fields")
	}
	if err := validateTimeRequired(v.AdGroup.ScheduleStart); err != nil {
		return err
	}
	if err := validateTimeOptional(v.AdGroup.ScheduleEnd); err != nil {
		return err
	}
	if err := validateScheduleOrder(v.AdGroup.ScheduleStart, v.AdGroup.ScheduleEnd); err != nil {
		return err
	}
	if v.Campaign.Objective == "WEB_CONVERSIONS" && (v.AdGroup.PixelID == "" || v.AdGroup.OptimizationEvent == "") {
		return errors.New("conversion campaign requires pixel and optimization event")
	}
	for _, ad := range v.Ads {
		if ad.Name == "" || ad.IdentityID == "" || ad.IdentityType == "" || ad.AssetID == "" || (ad.AssetKind != "image" && ad.AssetKind != "video") || ad.PrimaryText == "" || ad.CallToAction == "" || !validHTTPSURL(ad.DestinationURL) || ad.Status != "DISABLE" {
			return errors.New("invalid ad creative")
		}
		if err := validateName(ad.Name); err != nil {
			return err
		}
		if len([]rune(ad.PrimaryText)) > 100 {
			return errors.New("ad primary text is too long")
		}
	}
	return nil
}

func validateName(v string) error {
	length := len([]rune(strings.TrimSpace(v)))
	if length == 0 || length > 100 {
		return errors.New("name must contain 1 to 100 characters")
	}
	return nil
}

func validateScheduleOrder(start, end string) error {
	if start == "" || end == "" {
		return nil
	}
	const layout = "2006-01-02 15:04:05"
	startTime, startErr := time.Parse(layout, start)
	endTime, endErr := time.Parse(layout, end)
	if startErr != nil || endErr != nil || endTime.Before(startTime) {
		return errors.New("schedule end must not precede schedule start")
	}
	return nil
}

func overlaps(left, right []string) bool {
	seen := make(map[string]struct{}, len(left))
	for _, value := range left {
		seen[value] = struct{}{}
	}
	for _, value := range right {
		if _, ok := seen[value]; ok {
			return true
		}
	}
	return false
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func validateTimeRequired(v string) error {
	if _, err := time.Parse("2006-01-02 15:04:05", v); err != nil {
		return errors.New("invalid schedule time")
	}
	return nil
}
func validateTimeOptional(v string) error {
	if v == "" {
		return nil
	}
	return validateTimeRequired(v)
}
func validHTTPSURL(v string) bool {
	u, err := url.Parse(v)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil
}
