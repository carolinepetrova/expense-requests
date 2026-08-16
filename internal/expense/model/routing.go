package model

import (
	"github.com/carolinepetrova/expense-requests/internal/approval"
	"github.com/carolinepetrova/expense-requests/internal/user"
)

const (
	RoleManager  = "Manager"
	RoleFinance  = "Finance"
	RoleApprover = "Approver"
)

type Subject struct {
	Values    Values
	Requester user.User
	Manager   *user.User
	Finance   *user.User
}

// The two specs are built on each call rather than kept as package variables.
// A Spec holds a slice of rules, and a caller that appended to a shared one
// would silently change how every later request is routed.

// SingleStepSpec is the routing the exercise asks for: one approver, chosen by
// the amount.
func SingleStepSpec() approval.Spec[Subject] {
	return approval.Spec[Subject]{
		Rules: []approval.Rule[Subject]{
			{Name: RoleManager, When: amountUnder(ThresholdCents), Who: resolveManager},
			{Name: RoleFinance, When: amountAtLeast(ThresholdCents), Who: resolveFinance},
		},
		Fallback:     resolveFinance,
		FallbackName: RoleFinance,

		Requester: requesterOf,
	}
}

// MultiStepSpec is the optional extension: everything goes to the manager, and
// anything at or above the threshold then goes to finance as well. Only the
// first rule's condition differs from the single-step table.
func MultiStepSpec() approval.Spec[Subject] {
	return approval.Spec[Subject]{
		Rules: []approval.Rule[Subject]{
			{Name: RoleManager, When: always, Who: resolveManager},
			{Name: RoleFinance, When: amountAtLeast(ThresholdCents), Who: resolveFinance},
		},

		Fallback:     resolveFinance,
		FallbackName: RoleFinance,

		Requester: requesterOf,
	}
}

// Route compiles the approval chain for a subject, or reports that the request
// cannot be approved by anybody other than the person who raised it.
func Route(spec approval.Spec[Subject], s Subject) (approval.Chain, error) {
	return approval.Compile(spec, s)
}

func requesterOf(s Subject) approval.Approver {
	return approval.Approver(s.Requester.ID)
}

func always(Subject) bool { return true }

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
