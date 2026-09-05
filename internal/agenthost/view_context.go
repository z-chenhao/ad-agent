package agenthost

import (
	"errors"
	"strings"
	"time"

	"github.com/z-chenhao/ad-agent/internal/ads"
)

// ViewContext is a non-authoritative navigation hint from the product UI. It helps the
// agent resolve words such as "this" while object truth and mutation provenance still
// come only from backend reads.
type ViewContext struct {
	Page         string `json:"page"`
	AccountID    string `json:"account_id,omitempty"`
	AccountName  string `json:"account_name,omitempty"`
	EntityLevel  string `json:"entity_level,omitempty"`
	EntityID     string `json:"entity_id,omitempty"`
	EntityName   string `json:"entity_name,omitempty"`
	StartDate    string `json:"start_date,omitempty"`
	EndDate      string `json:"end_date,omitempty"`
	CompareStart string `json:"compare_start,omitempty"`
	CompareEnd   string `json:"compare_end,omitempty"`
}

func (v ViewContext) Empty() bool { return v.Page == "" }

func (v ViewContext) Validate() error {
	if v.Empty() {
		return nil
	}
	switch v.Page {
	case "today", "accounts", "campaigns", "creatives", "changes":
	default:
		return errors.New("invalid_view_page")
	}
	if len(v.AccountID) > 128 || len(v.AccountName) > 256 || len(v.EntityID) > 128 || len(v.EntityName) > 256 {
		return errors.New("view_context_too_large")
	}
	if strings.ContainsAny(v.AccountName+v.EntityName, "\x00\r\n") {
		return errors.New("invalid_view_context_text")
	}
	if v.EntityID == "" && v.EntityLevel != "" || v.EntityID != "" && v.EntityLevel == "" {
		return errors.New("incomplete_view_entity")
	}
	if v.EntityLevel != "" {
		level := ads.Level(v.EntityLevel)
		if !level.Valid() || level == ads.Advertiser {
			return errors.New("invalid_view_entity_level")
		}
	}
	for _, value := range []string{v.StartDate, v.EndDate, v.CompareStart, v.CompareEnd} {
		if value != "" {
			if _, err := time.Parse(time.DateOnly, value); err != nil {
				return errors.New("invalid_view_date")
			}
		}
	}
	if (v.StartDate == "") != (v.EndDate == "") || (v.CompareStart == "") != (v.CompareEnd == "") {
		return errors.New("incomplete_view_period")
	}
	return nil
}
