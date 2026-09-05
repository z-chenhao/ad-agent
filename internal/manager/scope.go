// Package manager routes an authorized set of advertiser accounts to independent
// account-scoped AdBackends. It never treats an advertiser ID as authority.
package manager

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/shopspring/decimal"
	"github.com/z-chenhao/ad-agent/internal/ads"
	"github.com/z-chenhao/ad-agent/internal/agenthost"
	"github.com/z-chenhao/ad-agent/internal/store"
)

const MaxAccounts = 50

type Binding struct {
	Backend  ads.Reader
	Writer   ads.Writer
	Creator  ads.Creator
	Planner  ads.OperationPlanner
	Operator ads.Operations
	Policy   ads.Policy
}

type Scope struct {
	ID       string
	Name     string
	store    *store.Store
	bindings map[string]Binding
}

type AccountSummary struct {
	Account     ads.Account      `json:"account"`
	Metrics     ads.Metrics      `json:"metrics"`
	ROAS        *decimal.Decimal `json:"roas"`
	Complete    bool             `json:"complete"`
	Limitations []string         `json:"limitations"`
}

type AccountSummaryReport struct {
	ID          string           `json:"id"`
	ScopeID     string           `json:"scope_id"`
	Start       string           `json:"start_date"`
	End         string           `json:"end_date"`
	Accounts    []AccountSummary `json:"accounts"`
	Limitations []string         `json:"limitations"`
	FetchedAt   time.Time        `json:"fetched_at"`
}

func NewScope(id, name string, s *store.Store, bindings []Binding) (*Scope, error) {
	if !validID(id) || name == "" || s == nil || len(bindings) == 0 || len(bindings) > MaxAccounts {
		return nil, errors.New("invalid manager scope")
	}
	p := &Scope{ID: id, Name: name, store: s, bindings: map[string]Binding{}}
	for _, binding := range bindings {
		if binding.Backend == nil {
			return nil, errors.New("manager binding requires a reader")
		}
		account, err := binding.Backend.Account(context.Background())
		if err != nil || account.ID == "" || account.Source.AccountID != account.ID {
			return nil, errors.New("invalid manager account binding")
		}
		if _, exists := p.bindings[account.ID]; exists {
			return nil, errors.New("duplicate manager advertiser")
		}
		p.bindings[account.ID] = binding
	}
	return p, nil
}

func (p *Scope) Source() ads.Source {
	return ads.Source{Backend: "manager", Environment: p.ID, AccountID: p.ID}
}

func (p *Scope) Accounts(ctx context.Context) ([]ads.Account, error) {
	accounts := make([]ads.Account, 0, len(p.bindings))
	for _, binding := range p.bindings {
		account, err := binding.Backend.Account(ctx)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].ID < accounts[j].ID })
	return accounts, nil
}

func (p *Scope) binding(accountID string) (Binding, error) {
	binding, ok := p.bindings[accountID]
	if !ok {
		return Binding{}, errors.New("advertiser_outside_manager")
	}
	return binding, nil
}

func (p *Scope) List(ctx context.Context, accountID string, query ads.EntityQuery) ([]ads.Entity, error) {
	binding, err := p.binding(accountID)
	if err != nil {
		return nil, err
	}
	return binding.Backend.List(ctx, query)
}

func (p *Scope) Get(ctx context.Context, accountID string, level ads.Level, id string) (ads.Entity, error) {
	binding, err := p.binding(accountID)
	if err != nil {
		return ads.Entity{}, err
	}
	entity, err := binding.Backend.Get(ctx, level, id)
	if err == nil && entity.AccountID != accountID {
		return ads.Entity{}, errors.New("advertiser_scope_mismatch")
	}
	return entity, err
}

func (p *Scope) Report(ctx context.Context, accountID string, query ads.ReportQuery) (ads.Report, error) {
	binding, err := p.binding(accountID)
	if err != nil {
		return ads.Report{}, err
	}
	return binding.Backend.Report(ctx, query)
}

func (p *Scope) AdDetail(ctx context.Context, accountID, adID string) (ads.AdDetail, error) {
	binding, err := p.binding(accountID)
	if err != nil {
		return ads.AdDetail{}, err
	}
	reader, ok := binding.Backend.(ads.AdDetailsReader)
	if !ok {
		return ads.AdDetail{}, ads.ErrNotFound
	}
	detail, err := reader.GetAdDetail(ctx, adID)
	if err == nil && detail.Ad.AccountID != accountID {
		return ads.AdDetail{}, errors.New("advertiser_scope_mismatch")
	}
	return detail, err
}

// Performance keeps every account's currency, timezone, completeness, and
// limitations separate. It deliberately does not create a cross-currency total.
func (p *Scope) Performance(ctx context.Context, start, end string) (AccountSummaryReport, error) {
	query := ads.ReportQuery{Level: ads.Advertiser, Start: start, End: end}
	if err := query.Validate(); err != nil {
		return AccountSummaryReport{}, err
	}
	accounts, err := p.Accounts(ctx)
	if err != nil {
		return AccountSummaryReport{}, err
	}
	result := AccountSummaryReport{ID: store.ID("account_summary"), ScopeID: p.ID, Start: start, End: end, Accounts: []AccountSummary{}, Limitations: []string{"Account results preserve their own currency, timezone, attribution, and coverage; no cross-currency total is calculated."}, FetchedAt: time.Now().UTC()}
	for _, account := range accounts {
		binding := p.bindings[account.ID]
		report, readErr := binding.Backend.Report(ctx, query)
		if readErr != nil {
			result.Accounts = append(result.Accounts, AccountSummary{Account: account, Complete: false, Limitations: []string{"Account report unavailable; inspect the host audit log for the private adapter error."}})
			continue
		}
		result.Accounts = append(result.Accounts, AccountSummary{Account: account, Metrics: report.Totals, ROAS: report.Totals.ROAS(), Complete: report.Complete, Limitations: report.Limitations})
	}
	return result, nil
}

func provenanceKey(accountID string, level ads.Level, id string) string {
	return accountID + "/" + string(level) + "/" + id
}

func (p *Scope) changes(accountID string) (agenthost.Changes, error) {
	binding, err := p.binding(accountID)
	if err != nil {
		return agenthost.Changes{}, err
	}
	return agenthost.Changes{Backend: binding.Backend, Writer: binding.Writer, Creator: binding.Creator, Planner: binding.Planner, Operator: binding.Operator, Store: p.store, Policy: binding.Policy}, nil
}

func (p *Scope) stage(ctx context.Context, managerSession store.Session, accountID string, before, after ads.Entity, kind ads.ChangeKind, reason string) (ads.Change, error) {
	changes, err := p.changes(accountID)
	if err != nil {
		return ads.Change{}, err
	}
	account, err := changes.Backend.Account(ctx)
	if err != nil {
		return ads.Change{}, err
	}
	seen, ok := managerSession.Provenance[provenanceKey(accountID, before.Level, before.ID)]
	if !ok {
		return ads.Change{}, errors.New("read_target_first")
	}
	accountSession := store.Session{ID: managerSession.ID, Source: account.Source, Provenance: map[string]store.Seen{before.ID: seen}}
	return changes.Stage(ctx, accountSession, before, after, kind, reason)
}

func (p *Scope) StageBudget(ctx context.Context, session store.Session, accountID string, level ads.Level, id string, amount decimal.Decimal, currency, reason string) (ads.Change, error) {
	entity, err := p.Get(ctx, accountID, level, id)
	if err != nil {
		return ads.Change{}, err
	}
	account, err := p.bindings[accountID].Backend.Account(ctx)
	if err != nil {
		return ads.Change{}, err
	}
	if currency != account.Currency {
		return ads.Change{}, errors.New("currency_mismatch")
	}
	after := entity
	after.Budget = &amount
	return p.stage(ctx, session, accountID, entity, after, ads.BudgetChange, reason)
}

func (p *Scope) StageStatus(ctx context.Context, session store.Session, accountID string, level ads.Level, id, status, reason string) (ads.Change, error) {
	entity, err := p.Get(ctx, accountID, level, id)
	if err != nil {
		return ads.Change{}, err
	}
	after := entity
	after.Status = status
	return p.stage(ctx, session, accountID, entity, after, ads.StatusChange, reason)
}

func (p *Scope) StageCreate(ctx context.Context, session store.Session, accountID string, request ads.CreateRequest, reason string) (ads.Change, error) {
	changes, err := p.changes(accountID)
	if err != nil {
		return ads.Change{}, err
	}
	account, err := changes.Backend.Account(ctx)
	if err != nil {
		return ads.Change{}, err
	}
	accountSession := store.Session{ID: session.ID, Source: account.Source, Provenance: map[string]store.Seen{}}
	if request.ParentID != "" {
		parentLevel := ads.Campaign
		if request.Level == ads.Ad {
			parentLevel = ads.AdGroup
		}
		seen, ok := session.Provenance[provenanceKey(accountID, parentLevel, request.ParentID)]
		if !ok {
			return ads.Change{}, errors.New("read_parent_first")
		}
		accountSession.Provenance[request.ParentID] = seen
	}
	return changes.StageCreate(ctx, accountSession, request, reason)
}

func (p *Scope) Apply(ctx context.Context, sessionID, changeID, operator string) (ads.Change, error) {
	change, err := p.store.Change(ctx, changeID)
	if err != nil {
		return change, err
	}
	changes, err := p.changes(change.Source.AccountID)
	if err != nil {
		return change, err
	}
	return changes.Apply(ctx, sessionID, changeID, operator)
}

func (p *Scope) Discard(ctx context.Context, sessionID, changeID string) (ads.Change, error) {
	change, err := p.store.Change(ctx, changeID)
	if err != nil {
		return change, err
	}
	changes, err := p.changes(change.Source.AccountID)
	if err != nil {
		return change, err
	}
	return changes.Discard(ctx, sessionID, changeID)
}

func (p *Scope) Reconcile(ctx context.Context, sessionID, changeID string) (ads.Change, error) {
	change, err := p.store.Change(ctx, changeID)
	if err != nil {
		return change, err
	}
	changes, err := p.changes(change.Source.AccountID)
	if err != nil {
		return change, err
	}
	return changes.Reconcile(ctx, sessionID, changeID)
}

func validID(value string) bool {
	if len(value) < 1 || len(value) > 64 {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return false
		}
	}
	return true
}
