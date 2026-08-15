package model

import (
	"github.com/carolinepetrova/expense-requests/internal/approval"
	"github.com/carolinepetrova/expense-requests/internal/user"
)

// Step names. They appear in the history and in the UI's progress strip, so
// they name the role the person acted in rather than the rule that produced
// them.
const (
	RoleManager = "Manager"
	RoleFinance = "Finance"

	// RoleApprover labels a step reconstructed from sample data, which records
	// an approver but not the role they were acting in.
	RoleApprover = "Approver"
)

// Subject is everything the routing rules need, resolved in advance.
//
// The rules are pure functions of this struct: no directory, no context, no
// error handling. Every lookup happens once in the service, where I/O belongs,
// which also means routing can be tested with a struct literal and no fakes.
type Subject struct {
	Values    Values
	Requester user.User

	// Manager is the requester's manager, nil when they have none or the
	// managerId points at somebody who has left.
	Manager *user.User

	// Finance is whoever holds the finance role, nil when nobody does.
	Finance *user.User
}

// Spec is this domain's approval policy.
//
// The two predicates are mutually exclusive, so exactly one step compiles: an
// expense goes to the requester's manager, or, at $1,000 and above, to finance.
// Making this multi-step is a change to this table alone — set the manager
// rule's When to fire always and both stages apply.
var Spec = approval.Spec[Subject]{
	Rules: []approval.Rule[Subject]{
		{Name: RoleManager, When: amountUnder(ThresholdCents), Who: resolveManager},
		{Name: RoleFinance, When: amountAtLeast(ThresholdCents), Who: resolveFinance},
	},

	// Anything that cannot be staffed falls to finance, and nobody ever
	// approves their own request. Both behaviours live in the engine; this
	// only says who the fallback is.
	Fallback:     resolveFinance,
	FallbackName: RoleFinance,

	Requester: func(s Subject) approval.Approver {
		return approval.Approver(s.Requester.ID)
	},
}

// Route compiles the approval chain for a subject, or reports that the request
// cannot be approved by anybody other than the person who raised it.
func Route(s Subject) (approval.Chain, error) {
	return approval.Compile(Spec, s)
}

func amountUnder(n int64) func(Subject) bool {
	return func(s Subject) bool { return s.Values.AmountCents < n }
}

func amountAtLeast(n int64) func(Subject) bool {
	return func(s Subject) bool { return s.Values.AmountCents >= n }
}

func resolveManager(s Subject) (approval.Approver, bool) {
	if s.Manager == nil {
		return "", false
	}
	return approval.Approver(s.Manager.ID), true
}

func resolveFinance(s Subject) (approval.Approver, bool) {
	if s.Finance == nil {
		return "", false
	}
	return approval.Approver(s.Finance.ID), true
}
