// Package ads defines the account-scoped domain independent of model runtimes and HTTP APIs.
package ads

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

type Level string

const (
	Advertiser Level = "advertiser"
	Campaign   Level = "campaign"
	AdGroup    Level = "ad_group"
	Ad         Level = "ad"
)

func (l Level) Valid() bool { return l == Advertiser || l == Campaign || l == AdGroup || l == Ad }

type Source struct {
	Backend     string `json:"backend"`
	Environment string `json:"environment"`
	AccountID   string `json:"account_id"`
}
type Account struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Currency    string   `json:"currency"`
	Timezone    string   `json:"timezone"`
	Source      Source   `json:"source"`
	LatestDate  string   `json:"latest_date"`
	Limitations []string `json:"limitations"`
}
type Entity struct {
	ID         string           `json:"id"`
	AccountID  string           `json:"account_id"`
	Level      Level            `json:"level"`
	ParentID   string           `json:"parent_id,omitempty"`
	Name       string           `json:"name"`
	Status     string           `json:"status"`
	Budget     *decimal.Decimal `json:"budget,omitempty"`
	BudgetMode string           `json:"budget_mode,omitempty"`
	Objective  string           `json:"objective,omitempty"`
}

func (e Entity) Version() string {
	b, _ := json.Marshal(e)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

type EntityQuery struct {
	Level    Level  `json:"level"`
	ParentID string `json:"parent_id,omitempty"`
}
type Metrics struct {
	Spend       decimal.Decimal  `json:"spend"`
	Impressions int64            `json:"impressions"`
	Clicks      int64            `json:"clicks"`
	Conversions *decimal.Decimal `json:"conversions"`
	Revenue     *decimal.Decimal `json:"revenue"`
}

func ZeroMetrics() Metrics { z := decimal.Zero; return Metrics{Conversions: &z, Revenue: &z} }
func (m Metrics) Add(n Metrics) Metrics {
	m.Spend = m.Spend.Add(n.Spend)
	m.Impressions += n.Impressions
	m.Clicks += n.Clicks
	if m.Conversions == nil || n.Conversions == nil {
		m.Conversions = nil
	} else {
		v := m.Conversions.Add(*n.Conversions)
		m.Conversions = &v
	}
	if m.Revenue == nil || n.Revenue == nil {
		m.Revenue = nil
	} else {
		v := m.Revenue.Add(*n.Revenue)
		m.Revenue = &v
	}
	return m
}
func Ratio(n *decimal.Decimal, d decimal.Decimal) *decimal.Decimal {
	if n == nil || d.IsZero() {
		return nil
	}
	v := n.Div(d)
	return &v
}
func (m Metrics) ROAS() *decimal.Decimal { return Ratio(m.Revenue, m.Spend) }

type Row struct {
	EntityID string  `json:"entity_id"`
	Date     string  `json:"date"`
	Metrics  Metrics `json:"metrics"`
}
type ReportQuery struct {
	Level    Level  `json:"level"`
	Start    string `json:"start_date"`
	End      string `json:"end_date"`
	EntityID string `json:"entity_id,omitempty"`
}

func (q ReportQuery) Validate() error {
	if !q.Level.Valid() {
		return errors.New("invalid report level")
	}
	a, err := time.Parse(time.DateOnly, q.Start)
	if err != nil {
		return errors.New("invalid start_date")
	}
	b, err := time.Parse(time.DateOnly, q.End)
	if err != nil {
		return errors.New("invalid end_date")
	}
	if b.Before(a) || b.Sub(a) > 92*24*time.Hour {
		return errors.New("report window must be 1–93 inclusive days")
	}
	return nil
}

type Report struct {
	ID          string      `json:"id"`
	Source      Source      `json:"source"`
	Query       ReportQuery `json:"query"`
	Currency    string      `json:"currency"`
	Timezone    string      `json:"timezone"`
	Attribution string      `json:"attribution"`
	FetchedAt   time.Time   `json:"fetched_at"`
	Complete    bool        `json:"complete"`
	Limitations []string    `json:"limitations"`
	Rows        []Row       `json:"rows"`
	Totals      Metrics     `json:"totals"`
	RequestIDs  []string    `json:"request_ids,omitempty"`
}
type Backend interface {
	Account(context.Context) (Account, error)
	List(context.Context, EntityQuery) ([]Entity, error)
	Get(context.Context, Level, string) (Entity, error)
	Report(context.Context, ReportQuery) (Report, error)
}
type WriteRequest struct {
	Target Entity           `json:"target"`
	Kind   string           `json:"kind"`
	Budget *decimal.Decimal `json:"budget,omitempty"`
	Status string           `json:"status,omitempty"`
}
type WriteOutcome struct {
	State     string `json:"state"` // acknowledged | rejected | unknown | not_sent
	RequestID string `json:"request_id,omitempty"`
	Message   string `json:"message,omitempty"`
}

// Writer is only supplied to the host change service, never to an agent runtime.
type Writer interface {
	Write(context.Context, WriteRequest) WriteOutcome
}

var ErrNotFound = errors.New("entity not found in the bound account")

func CheckContext(ctx context.Context) error { return ctx.Err() }
func CheckEntity(e Entity, account string) error {
	if e.ID == "" || e.AccountID != account || !e.Level.Valid() || e.Level == Advertiser {
		return fmt.Errorf("invalid account-scoped entity %q", e.ID)
	}
	if e.Status != "ENABLE" && e.Status != "DISABLE" {
		return errors.New("unsupported operation status")
	}
	if e.Budget != nil && e.Budget.IsNegative() {
		return errors.New("negative budget")
	}
	return nil
}
