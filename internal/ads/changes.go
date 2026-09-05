package ads

import (
	"errors"
	"github.com/shopspring/decimal"
	"time"
)

type ChangeKind string

const (
	BudgetChange    ChangeKind = "budget"
	StatusChange    ChangeKind = "status"
	CreateChange    ChangeKind = "create"
	OperationChange ChangeKind = "operation"
)

func CanTransition(from, to ChangeState) bool {
	switch from {
	case Staged:
		return to == Applying || to == Discarded || to == Expired
	case Applying:
		return to == Applied || to == Failed || to == Indeterminate || to == Expired
	case Indeterminate:
		return to == Applied || to == Failed
	}
	return false
}

type ChangeState string

const (
	Staged        ChangeState = "staged"
	Applying      ChangeState = "applying"
	Applied       ChangeState = "applied"
	Discarded     ChangeState = "discarded"
	Expired       ChangeState = "expired"
	Failed        ChangeState = "failed"
	Indeterminate ChangeState = "indeterminate"
)

type Change struct {
	ID               string            `json:"id"`
	SessionID        string            `json:"session_id"`
	Source           Source            `json:"source"`
	Kind             ChangeKind        `json:"kind"`
	Before           *Entity           `json:"before,omitempty"`
	After            *Entity           `json:"after,omitempty"`
	Parent           *Entity           `json:"parent,omitempty"`
	Create           *CreateRequest    `json:"create,omitempty"`
	Created          *Entity           `json:"created,omitempty"`
	State            ChangeState       `json:"state"`
	Reason           string            `json:"reason"`
	Currency         string            `json:"currency"`
	SpendIncreasing  bool              `json:"spend_increasing"`
	CreatedAt        time.Time         `json:"created_at"`
	ExpiresAt        time.Time         `json:"expires_at"`
	ApprovedAt       *time.Time        `json:"approved_at,omitempty"`
	ApprovedBy       string            `json:"approved_by,omitempty"`
	AttemptID        string            `json:"attempt_id,omitempty"`
	Outcome          *WriteOutcome     `json:"outcome,omitempty"`
	Operation        *OperationPlan    `json:"operation,omitempty"`
	OperationOutcome *OperationOutcome `json:"operation_outcome,omitempty"`
	Note             string            `json:"note,omitempty"`
}

// Policy is host configuration, never supplied by the model or a change preview.
type Policy struct {
	MaxBudget, MaxDeltaPercent, MinBudget decimal.Decimal
	LiveWrites                            bool
}

func SandboxPolicy() Policy {
	return Policy{MaxBudget: decimal.NewFromInt(50000), MaxDeltaPercent: decimal.NewFromInt(20), MinBudget: decimal.NewFromInt(1)}
}

// ReadOnlyPolicy keeps all real writes disabled and intentionally leaves live
// budget guardrails unconfigured. It may be replaced only by explicit operator policy.
func ReadOnlyPolicy() Policy { return Policy{} }
func (p Policy) Validate(before, after Entity, kind ChangeKind) error {
	if before.ID != after.ID || before.AccountID != after.AccountID || before.Level != after.Level {
		return errors.New("target_mismatch")
	}
	if err := CheckEntity(after, before.AccountID); err != nil {
		return errors.New("invalid_target")
	}
	expected := before
	switch kind {
	case BudgetChange:
		if before.Level == Ad || before.Budget == nil || after.Budget == nil {
			return errors.New("budget_not_supported")
		}
		if before.Budget.Equal(*after.Budget) {
			return errors.New("no_change")
		}
		if !p.MaxBudget.IsPositive() || !p.MaxDeltaPercent.IsPositive() || !p.MinBudget.IsPositive() {
			return errors.New("budget_policy_not_configured")
		}
		if after.Budget.GreaterThan(p.MaxBudget) || after.Budget.LessThan(p.MinBudget) {
			return errors.New("budget_outside_limits")
		}
		if !before.Budget.IsPositive() || after.Budget.Sub(*before.Budget).Abs().Mul(decimal.NewFromInt(100)).GreaterThan(before.Budget.Mul(p.MaxDeltaPercent)) {
			return errors.New("budget_delta_exceeded")
		}
		expected.Budget = after.Budget
	case StatusChange:
		if before.Status == after.Status {
			return errors.New("no_change")
		}
		expected.Status = after.Status
	default:
		return errors.New("unsupported_change")
	}
	if expected.Version() != after.Version() {
		return errors.New("multiple_fields_not_allowed")
	}
	return nil
}
