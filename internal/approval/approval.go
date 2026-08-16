package approval

//go:generate go-enum --marshal --values -f=$GOFILE

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

var (
	ErrNoEligibleApprover = errors.New("no eligible approver")
	ErrNotCurrentStep     = errors.New("not the current step")
	ErrNotTheApprover     = errors.New("not the assigned approver")
)

type Approver string

// ENUM(Pending, Approved, Rejected).
type StepStatus string

// Step is one stage of a chain: a named role, the person it resolved to, and
// the decision they made (if any).
type Step struct {
	Name       string     `json:"name"`
	ApproverID Approver   `json:"approverId"`
	Status     StepStatus `json:"status"`
	Comment    string     `json:"comment,omitempty"`
	ActedAt    *time.Time `json:"actedAt,omitempty"`
}

// Chain is an ordered list of steps. Position matters: a step is only
// actionable once every step before it has been approved.
type Chain []Step

type Rule[T any] struct {
	Name string
	When func(T) bool
	Who  func(T) (Approver, bool)
}

// Spec is a complete routing policy for subject type T.
type Spec[T any] struct {
	Rules        []Rule[T]
	Fallback     func(T) (Approver, bool)
	FallbackName string
	Requester    func(T) Approver
}

func Compile[T any](s Spec[T], subject T) (Chain, error) {
	requester := s.Requester(subject)

	chain := make(Chain, 0, len(s.Rules))
	for _, rule := range s.Rules {
		if !rule.When(subject) {
			continue
		}

		step, ok := s.resolve(rule, subject, requester)
		if !ok {
			continue
		}

		// Two rules that land on the same person are one approval, not two:
		// nobody should be asked to sign the same request twice in a row.
		if n := len(chain); n > 0 && chain[n-1].ApproverID == step.ApproverID {
			continue
		}
		chain = append(chain, step)
	}

	if len(chain) == 0 {
		return nil, ErrNoEligibleApprover
	}
	return chain, nil
}

// resolve turns one matching rule into the step it contributes.
//
// A rule that cannot name anybody, or that names the requester, falls back to
// the spec's fallback approver; if that is also the requester the rule drops
// out entirely, which is how a chain ends up empty.
func (s Spec[T]) resolve(rule Rule[T], subject T, requester Approver) (Step, bool) {
	if who, ok := rule.Who(subject); ok && who != "" && who != requester {
		return Step{Name: rule.Name, ApproverID: who, Status: StepStatusPending}, true
	}

	who, ok := s.Fallback(subject)
	if !ok || who == requester {
		return Step{}, false
	}
	return Step{Name: s.FallbackName, ApproverID: who, Status: StepStatusPending}, true
}

// Current returns the step awaiting a decision.
//
// A rejected chain is waiting on nobody, even though the steps after the
// rejection are still Pending — they were never reached rather than skipped.
// Without this, a rejected request would keep naming its next approver and
// turn up in that person's queue.
func (c Chain) Current() (int, Step, bool) {
	if c.Rejected() {
		return -1, Step{}, false
	}

	for i, s := range c {
		if s.Status == StepStatusPending {
			return i, s, true
		}
	}
	return -1, Step{}, false
}

func (c Chain) Rejected() bool {
	return slices.ContainsFunc(c, func(s Step) bool {
		return s.Status == StepStatusRejected
	})
}

func (c Chain) Complete() bool {
	_, _, pending := c.Current()
	return !pending && !c.Rejected()
}

func (c Chain) IsCurrentApprover(actor Approver) bool {
	_, step, ok := c.Current()
	return ok && step.ApproverID == actor
}

func (c Chain) Approve(i int, by Approver, comment string, at time.Time) (Chain, error) {
	return c.decide(i, by, StepStatusApproved, comment, at)
}

func (c Chain) Reject(i int, by Approver, comment string, at time.Time) (Chain, error) {
	return c.decide(i, by, StepStatusRejected, comment, at)
}

func (c Chain) decide(
	i int,
	by Approver,
	outcome StepStatus,
	comment string,
	at time.Time,
) (Chain, error) {
	cur, _, ok := c.Current()
	if !ok || cur != i {
		return nil, fmt.Errorf("%w: step %d", ErrNotCurrentStep, i)
	}
	if c[i].ApproverID != by {
		return nil, ErrNotTheApprover
	}

	out := slices.Clone(c)
	out[i].Status = outcome
	out[i].Comment = comment
	out[i].ActedAt = &at

	return out, nil
}
