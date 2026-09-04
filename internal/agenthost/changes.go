package agenthost

import (
	"context"
	"errors"
	"github.com/z-chenhao/ad-agent/internal/ads"
	"github.com/z-chenhao/ad-agent/internal/store"
	"time"
)

type Changes struct {
	Backend ads.Reader
	Writer  ads.Writer
	Creator ads.Creator
	Store   *store.Store
	Policy  ads.Policy
}

func (s Changes) Stage(ctx context.Context, session store.Session, before, after ads.Entity, kind ads.ChangeKind, reason string) (ads.Change, error) {
	seen, ok := session.Provenance[before.ID]
	if !ok || time.Since(seen.At) > 15*time.Minute {
		return ads.Change{}, errors.New("read_target_first")
	}
	current, err := s.Backend.Get(ctx, before.Level, before.ID)
	if err != nil {
		return ads.Change{}, err
	}
	if current.Version() != seen.Entity.Version() || current.Version() != before.Version() {
		return ads.Change{}, errors.New("target_changed_read_again")
	}
	if err = s.Policy.Validate(current, after, kind); err != nil {
		return ads.Change{}, err
	}
	a, err := s.Backend.Account(ctx)
	if err != nil {
		return ads.Change{}, err
	}
	if a.Source != session.Source {
		return ads.Change{}, errors.New("source_mismatch")
	}
	c := ads.Change{ID: store.ID("change"), SessionID: session.ID, Source: a.Source, Kind: kind, Before: &current, After: &after, State: ads.Staged, Reason: reason, Currency: a.Currency, CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(15 * time.Minute)}
	c.SpendIncreasing = kind == ads.StatusChange && after.Status == "ENABLE" || kind == ads.BudgetChange && after.Budget.GreaterThan(*before.Budget)
	err = s.Store.InsertChange(ctx, c)
	return c, err
}

func (s Changes) StageCreate(ctx context.Context, session store.Session, request ads.CreateRequest, reason string) (ads.Change, error) {
	if s.Creator == nil {
		return ads.Change{}, errors.New("create_unavailable")
	}
	if request.Validate() != nil {
		return ads.Change{}, errors.New("invalid_create_request")
	}
	var parent *ads.Entity
	if request.Level == ads.Campaign {
		if request.ParentID != "" {
			return ads.Change{}, errors.New("invalid_create_parent")
		}
	} else {
		seen, ok := session.Provenance[request.ParentID]
		if !ok || time.Since(seen.At) > 15*time.Minute {
			return ads.Change{}, errors.New("read_parent_first")
		}
		want := ads.Campaign
		if request.Level == ads.Ad {
			want = ads.AdGroup
		}
		if seen.Entity.Level != want {
			return ads.Change{}, errors.New("invalid_create_parent")
		}
		current, err := s.Backend.Get(ctx, want, request.ParentID)
		if err != nil || current.Version() != seen.Entity.Version() {
			return ads.Change{}, errors.New("parent_changed_read_again")
		}
		parent = &current
	}
	a, err := s.Backend.Account(ctx)
	if err != nil {
		return ads.Change{}, err
	}
	if a.Source != session.Source {
		return ads.Change{}, errors.New("source_mismatch")
	}
	c := ads.Change{ID: store.ID("change"), SessionID: session.ID, Source: a.Source, Kind: ads.CreateChange, Parent: parent, Create: &request, State: ads.Staged, Reason: reason, Currency: a.Currency, SpendIncreasing: request.Status == "ENABLE", CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(15 * time.Minute)}
	return c, s.Store.InsertChange(ctx, c)
}

// Apply is only called by an explicit CLI command or authenticated host approval route.
func (s Changes) Apply(ctx context.Context, sessionID, id, operator string) (ads.Change, error) {
	c, err := s.Store.Change(ctx, id)
	if err != nil {
		return c, err
	}
	if c.SessionID != sessionID || operator == "" {
		return c, errors.New("approval_scope_mismatch")
	}
	if c.State != ads.Staged {
		return c, errors.New("change_not_staged")
	}
	if !c.ExpiresAt.After(time.Now()) {
		c.State = ads.Expired
		err = s.Store.Transition(ctx, ads.Staged, c)
		return c, err
	}
	a, err := s.Backend.Account(ctx)
	if err != nil {
		return c, err
	}
	if a.Source != c.Source {
		return c, errors.New("source_mismatch")
	}
	if a.Source.Backend != "sandbox" && !s.Policy.LiveWrites {
		return c, errors.New("live_writes_disabled")
	}
	if c.Kind == ads.CreateChange && s.Creator == nil || c.Kind != ads.CreateChange && s.Writer == nil {
		return c, errors.New("writer_unavailable")
	}
	leaseID := "apply:" + a.Source.Backend + ":" + a.Source.Environment + ":" + a.ID
	leaseOwner := store.ID("execution")
	if err = s.Store.Lease(ctx, leaseID, leaseOwner, time.Now().Add(time.Minute)); err != nil {
		return c, err
	}
	defer s.Store.Release(leaseID, leaseOwner)
	now := time.Now().UTC()
	c.ApprovedAt = &now
	c.ApprovedBy = operator
	c.AttemptID = store.ID("attempt")
	c.State = ads.Applying
	if err = s.Store.Transition(ctx, ads.Staged, c); err != nil {
		return c, err
	}
	// Once claimed, client disconnects do not make the remote result disappear.
	applyCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if c.Kind == ads.CreateChange {
		s.applyCreate(applyCtx, &c)
	} else if c.Before == nil || c.After == nil {
		c.State = ads.Failed
		c.Note = "invalid_change_payload"
	} else {
		current, readErr := s.Backend.Get(applyCtx, c.Before.Level, c.Before.ID)
		if readErr != nil {
			c.State = ads.Failed
			c.Note = "revalidation_read_failed; request not sent"
		} else if current.Version() != c.Before.Version() {
			c.State = ads.Expired
			c.Note = "target_changed"
		} else if err = s.Policy.Validate(current, *c.After, c.Kind); err != nil {
			c.State = ads.Failed
			c.Note = err.Error()
		} else {
			outcome := s.Writer.Write(applyCtx, ads.WriteRequest{Target: current, Kind: string(c.Kind), Budget: c.After.Budget, Status: c.After.Status})
			c.Outcome = &outcome
			switch outcome.State {
			case "not_sent", "rejected":
				c.State = ads.Failed
			case "acknowledged":
				observed, e := s.Backend.Get(applyCtx, c.Before.Level, c.Before.ID)
				if e == nil && observed.Version() == c.After.Version() {
					c.State = ads.Applied
					c.Note = "acknowledged and read-back matched"
				} else {
					c.State = ads.Indeterminate
					c.Note = "read-back not confirmed"
				}
			default:
				c.State = ads.Indeterminate
				c.Note = "remote outcome unknown; no automatic retry"
			}
		}
	}
	saveCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	err = s.Store.Transition(saveCtx, ads.Applying, c)
	return c, err
}

func (s Changes) applyCreate(ctx context.Context, change *ads.Change) {
	if change.Create == nil {
		change.State = ads.Failed
		change.Note = "invalid_create_payload"
		return
	}
	if change.Parent != nil {
		current, err := s.Backend.Get(ctx, change.Parent.Level, change.Parent.ID)
		if err != nil || current.Version() != change.Parent.Version() {
			change.State = ads.Expired
			change.Note = "parent_changed"
			return
		}
	}
	created, err := s.Creator.Create(ctx, *change.Create)
	if err != nil {
		change.State = ads.Failed
		change.Note = "create_failed"
		return
	}
	change.Created = &created
	outcome := ads.WriteOutcome{State: "acknowledged", RequestID: "sandbox-create"}
	change.Outcome = &outcome
	observed, err := s.Backend.Get(ctx, created.Level, created.ID)
	if err != nil || observed.Version() != created.Version() {
		change.State = ads.Indeterminate
		change.Note = "creation acknowledged; read-back not confirmed"
		return
	}
	change.State = ads.Applied
	change.Note = "created and read-back matched"
}

// Reconcile observes current state without retrying a possibly sent write.
func (s Changes) Reconcile(ctx context.Context, sessionID, id string) (ads.Change, error) {
	c, err := s.Store.Change(ctx, id)
	if err != nil {
		return c, err
	}
	if c.SessionID != sessionID {
		return c, errors.New("change_scope_mismatch")
	}
	a, err := s.Backend.Account(ctx)
	if err != nil {
		return c, err
	}
	if a.Source != c.Source {
		return c, errors.New("source_mismatch")
	}
	if c.State == ads.Applying {
		// Approval execution is bounded to 30 seconds plus persistence; do not steal a live attempt.
		if c.ApprovedAt == nil || time.Since(*c.ApprovedAt) < time.Minute {
			return c, errors.New("execution_may_be_active")
		}
		c.State = ads.Indeterminate
		c.Note = "interrupted execution; request may have been sent"
		if err = s.Store.Transition(ctx, ads.Applying, c); err != nil {
			return c, err
		}
	}
	if c.State != ads.Indeterminate {
		return c, errors.New("change_not_indeterminate")
	}
	if c.Kind == ads.CreateChange {
		if c.Created == nil {
			return c, errors.New("change_not_reconcilable")
		}
		observed, observeErr := s.Backend.Get(ctx, c.Created.Level, c.Created.ID)
		if observeErr != nil || observed.Version() != c.Created.Version() {
			return c, nil
		}
		c.State = ads.Applied
		c.Note = "created object now matches; execution attribution unconfirmed"
		err = s.Store.Transition(ctx, ads.Indeterminate, c)
		return c, err
	}
	if c.Before == nil || c.After == nil {
		return c, errors.New("change_not_reconcilable")
	}
	observed, err := s.Backend.Get(ctx, c.Before.Level, c.Before.ID)
	if err != nil {
		return c, err
	}
	if observed.Version() != c.After.Version() {
		return c, nil
	}
	c.State = ads.Applied
	c.Note = "current state matches requested after; execution attribution unconfirmed"
	err = s.Store.Transition(ctx, ads.Indeterminate, c)
	return c, err
}
func (s Changes) Discard(ctx context.Context, sessionID, id string) (ads.Change, error) {
	c, err := s.Store.Change(ctx, id)
	if err != nil {
		return c, err
	}
	if c.SessionID != sessionID {
		return c, errors.New("change_scope_mismatch")
	}
	if c.State != ads.Staged {
		return c, errors.New("change_not_staged")
	}
	c.State = ads.Discarded
	err = s.Store.Transition(ctx, ads.Staged, c)
	return c, err
}
