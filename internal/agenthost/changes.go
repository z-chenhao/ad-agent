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
	c := ads.Change{ID: store.ID("change"), SessionID: session.ID, Source: a.Source, Kind: kind, Before: current, After: after, State: ads.Staged, Reason: reason, Currency: a.Currency, CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(15 * time.Minute)}
	c.SpendIncreasing = kind == ads.StatusChange && after.Status == "ENABLE" || kind == ads.BudgetChange && after.Budget.GreaterThan(*before.Budget)
	err = s.Store.InsertChange(ctx, c)
	return c, err
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
	if a.Source.Environment != "fixture" && !s.Policy.LiveWrites {
		return c, errors.New("live_writes_disabled")
	}
	if s.Writer == nil {
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
	current, err := s.Backend.Get(applyCtx, c.Before.Level, c.Before.ID)
	if err != nil {
		c.State = ads.Failed
		c.Note = "revalidation_read_failed; request not sent"
	} else if current.Version() != c.Before.Version() {
		c.State = ads.Expired
		c.Note = "target_changed"
	} else if err = s.Policy.Validate(current, c.After, c.Kind); err != nil {
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
	saveCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	err = s.Store.Transition(saveCtx, ads.Applying, c)
	return c, err
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
