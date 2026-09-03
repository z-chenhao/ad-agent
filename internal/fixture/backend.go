// Package fixture implements AdBackend over explicit official-example-derived lab data.
package fixture

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/z-chenhao/ad-agent/internal/ads"
)

//go:embed data/*.json
var files embed.FS

type wireEntity struct {
	AdvertiserID string           `json:"advertiser_id"`
	CampaignID   string           `json:"campaign_id"`
	AdGroupID    string           `json:"adgroup_id"`
	AdID         string           `json:"ad_id"`
	CampaignName string           `json:"campaign_name"`
	AdGroupName  string           `json:"adgroup_name"`
	AdName       string           `json:"ad_name"`
	Budget       *decimal.Decimal `json:"budget"`
	BudgetMode   string           `json:"budget_mode"`
	Status       string           `json:"operation_status"`
	Objective    string           `json:"objective_type"`
}
type WireRow struct {
	Dimensions struct {
		AdID string `json:"ad_id"`
		Date string `json:"stat_time_day"`
	} `json:"dimensions"`
	Metrics struct {
		Spend       decimal.Decimal  `json:"spend"`
		Impressions int64            `json:"impressions,string"`
		Clicks      int64            `json:"clicks,string"`
		Conversions *decimal.Decimal `json:"conversion"`
		Revenue     *decimal.Decimal `json:"total_purchase_value"`
	} `json:"metrics"`
}
type envelope[T any] struct {
	Data struct {
		List []T `json:"list"`
	} `json:"data"`
}
type Document struct {
	Account struct {
		ID       string `json:"advertiser_id"`
		Name     string `json:"advertiser_name"`
		Currency string `json:"currency"`
		Timezone string `json:"timezone"`
		Latest   string `json:"latest_date"`
	} `json:"account"`
	Campaigns envelope[wireEntity] `json:"campaigns"`
	AdGroups  envelope[wireEntity] `json:"adgroups"`
	Ads       envelope[wireEntity] `json:"ads"`
	Report    envelope[WireRow]    `json:"report"`
}
type Backend struct {
	mu       sync.RWMutex
	account  ads.Account
	entities map[string]ads.Entity
	rows     []ads.Row
}

func New() (*Backend, error) {
	b, err := files.ReadFile("data/mock.json")
	if err != nil {
		return nil, err
	}
	return FromJSON(b)
}
func FromJSON(b []byte) (*Backend, error) {
	var d Document
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	if d.Account.ID == "" || len(d.Account.Currency) != 3 {
		return nil, errors.New("invalid account metadata")
	}
	if _, err := time.LoadLocation(d.Account.Timezone); err != nil {
		return nil, errors.New("invalid account timezone")
	}
	source := ads.Source{Backend: "fixture", Environment: "fixture", AccountID: d.Account.ID}
	f := &Backend{account: ads.Account{ID: d.Account.ID, Name: d.Account.Name, Currency: d.Account.Currency, Timezone: d.Account.Timezone, LatestDate: d.Account.Latest, Source: source, Limitations: []string{"官方请求示例字段补充了合成投放数据；不是真实广告账户。", "固定历史窗口：2022-07-04 至 2022-07-17；购买价值及归因是合成数据。"}}, entities: map[string]ads.Entity{}}
	for _, group := range []struct {
		level ads.Level
		items []wireEntity
	}{{ads.Campaign, d.Campaigns.Data.List}, {ads.AdGroup, d.AdGroups.Data.List}, {ads.Ad, d.Ads.Data.List}} {
		for _, w := range group.items {
			e := ads.Entity{AccountID: w.AdvertiserID, Level: group.level, Status: w.Status, Budget: w.Budget, BudgetMode: w.BudgetMode, Objective: w.Objective}
			switch group.level {
			case ads.Campaign:
				e.ID = w.CampaignID
				e.Name = w.CampaignName
			case ads.AdGroup:
				e.ID = w.AdGroupID
				e.Name = w.AdGroupName
				e.ParentID = w.CampaignID
			case ads.Ad:
				e.ID = w.AdID
				e.Name = w.AdName
				e.ParentID = w.AdGroupID
			}
			if err := ads.CheckEntity(e, f.account.ID); err != nil {
				return nil, err
			}
			if _, ok := f.entities[e.ID]; ok {
				return nil, errors.New("duplicate entity ID")
			}
			f.entities[e.ID] = e
		}
	}
	for _, e := range f.entities {
		if e.Level != ads.Campaign {
			p, ok := f.entities[e.ParentID]
			if !ok || (e.Level == ads.Ad && p.Level != ads.AdGroup) || (e.Level == ads.AdGroup && p.Level != ads.Campaign) {
				return nil, errors.New("broken entity hierarchy")
			}
		}
	}
	seen := map[string]bool{}
	for _, w := range d.Report.Data.List {
		e, ok := f.entities[w.Dimensions.AdID]
		if !ok || e.Level != ads.Ad {
			return nil, errors.New("report references unknown ad")
		}
		if _, err := time.Parse(time.DateOnly, w.Dimensions.Date); err != nil {
			return nil, err
		}
		key := e.ID + "/" + w.Dimensions.Date
		if seen[key] {
			return nil, errors.New("duplicate ad/day")
		}
		seen[key] = true
		m := ads.Metrics{Spend: w.Metrics.Spend, Impressions: w.Metrics.Impressions, Clicks: w.Metrics.Clicks, Conversions: w.Metrics.Conversions, Revenue: w.Metrics.Revenue}
		if m.Spend.IsNegative() || m.Impressions < 0 || m.Clicks < 0 || m.Clicks > m.Impressions || m.Conversions != nil && m.Conversions.IsNegative() || m.Revenue != nil && m.Revenue.IsNegative() {
			return nil, errors.New("inconsistent metrics")
		}
		f.rows = append(f.rows, ads.Row{EntityID: e.ID, Date: w.Dimensions.Date, Metrics: m})
	}
	return f, nil
}
func (f *Backend) Account(ctx context.Context) (ads.Account, error) {
	if err := ctx.Err(); err != nil {
		return ads.Account{}, err
	}
	return f.account, nil
}
func (f *Backend) List(ctx context.Context, q ads.EntityQuery) ([]ads.Entity, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !q.Level.Valid() || q.Level == ads.Advertiser {
		return nil, errors.New("invalid entity level")
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := []ads.Entity{}
	for _, e := range f.entities {
		if e.Level == q.Level && (q.ParentID == "" || e.ParentID == q.ParentID) {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func (f *Backend) Get(ctx context.Context, l ads.Level, id string) (ads.Entity, error) {
	if err := ctx.Err(); err != nil {
		return ads.Entity{}, err
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	e, ok := f.entities[id]
	if !ok || e.Level != l {
		return ads.Entity{}, ads.ErrNotFound
	}
	return e, nil
}
func (f *Backend) Report(ctx context.Context, q ads.ReportQuery) (ads.Report, error) {
	if err := q.Validate(); err != nil {
		return ads.Report{}, err
	}
	if err := ctx.Err(); err != nil {
		return ads.Report{}, err
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	r := ads.Report{Source: f.account.Source, Query: q, Currency: f.account.Currency, Timezone: f.account.Timezone, Attribution: "fixture: synthetic purchase value; not a verified MAPI revenue mapping", FetchedAt: time.Now().UTC(), Complete: q.Start >= "2022-07-04" && q.End <= "2022-07-17", Limitations: append([]string{}, f.account.Limitations...), Rows: []ads.Row{}, Totals: ads.ZeroMetrics()}
	if !r.Complete {
		r.Limitations = append(r.Limitations, "请求日期超出合成数据覆盖范围。")
	}
	if q.EntityID != "" && q.Level != ads.Advertiser {
		e, ok := f.entities[q.EntityID]
		if !ok || e.Level != q.Level {
			return ads.Report{}, ads.ErrNotFound
		}
	}
	if q.Level == ads.Advertiser && q.EntityID != "" && q.EntityID != f.account.ID {
		return ads.Report{}, ads.ErrNotFound
	}
	coverage := map[string]bool{}
	for _, row := range f.rows {
		coverage[row.EntityID+"/"+row.Date] = true
	}
	start, _ := time.Parse(time.DateOnly, q.Start)
	end, _ := time.Parse(time.DateOnly, q.End)
	for _, entity := range f.entities {
		if entity.Level != ads.Ad {
			continue
		}
		e := entity
		for e.Level != q.Level && q.Level != ads.Advertiser {
			e = f.entities[e.ParentID]
		}
		if q.EntityID != "" && q.Level != ads.Advertiser && e.ID != q.EntityID {
			continue
		}
		for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
			if !coverage[entity.ID+"/"+day.Format(time.DateOnly)] {
				r.Complete = false
			}
		}
	}
	if !r.Complete {
		r.Limitations = append(r.Limitations, "缺少广告/日期覆盖；缺失行不能视为已确认的零值。")
	}
	groups := map[string]ads.Row{}
	for _, row := range f.rows {
		if row.Date < q.Start || row.Date > q.End {
			continue
		}
		e := f.entities[row.EntityID]
		for e.Level != q.Level && q.Level != ads.Advertiser {
			if e.ParentID == "" {
				return ads.Report{}, fmt.Errorf("cannot aggregate level %s", q.Level)
			}
			e = f.entities[e.ParentID]
		}
		id := e.ID
		if q.Level == ads.Advertiser {
			id = f.account.ID
		}
		if q.EntityID != "" && q.EntityID != id {
			continue
		}
		key := id + "/" + row.Date
		v, ok := groups[key]
		if !ok {
			v = ads.Row{EntityID: id, Date: row.Date, Metrics: ads.ZeroMetrics()}
		}
		v.Metrics = v.Metrics.Add(row.Metrics)
		groups[key] = v
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		r.Rows = append(r.Rows, groups[k])
		r.Totals = r.Totals.Add(groups[k].Metrics)
	}
	return r, nil
}

// Restore only accepts mutable fixture fields for existing identities; never imports live entities.
func (f *Backend) Restore(e ads.Entity) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	old, ok := f.entities[e.ID]
	if !ok {
		return ads.ErrNotFound
	}
	if err := ads.CheckEntity(e, f.account.ID); err != nil {
		return err
	}
	expected := old
	expected.Budget = e.Budget
	expected.Status = e.Status
	if expected.Version() != e.Version() {
		return errors.New("invalid fixture override")
	}
	f.entities[e.ID] = e
	return nil
}
func (f *Backend) Write(ctx context.Context, w ads.WriteRequest) ads.WriteOutcome {
	if ctx.Err() != nil {
		return ads.WriteOutcome{State: "not_sent", Message: "cancelled"}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.entities[w.Target.ID]
	if !ok || e.Version() != w.Target.Version() {
		return ads.WriteOutcome{State: "rejected", Message: "target changed"}
	}
	switch w.Kind {
	case "budget":
		if w.Budget == nil || e.Level == ads.Ad || w.Budget.IsNegative() {
			return ads.WriteOutcome{State: "rejected", Message: "invalid budget"}
		}
		v := *w.Budget
		e.Budget = &v
	case "status":
		if w.Status != "ENABLE" && w.Status != "DISABLE" {
			return ads.WriteOutcome{State: "rejected", Message: "invalid status"}
		}
		e.Status = w.Status
	default:
		return ads.WriteOutcome{State: "rejected", Message: "unsupported change"}
	}
	f.entities[e.ID] = e
	return ads.WriteOutcome{State: "acknowledged", RequestID: "fixture-write"}
}
