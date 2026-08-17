package model_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/carolinepetrova/expense-requests/internal/approval"
	"github.com/carolinepetrova/expense-requests/internal/expense/model"
	"github.com/carolinepetrova/expense-requests/internal/user"
)

// The sample organisation. Peggy is at the top with no manager, Trent is the
// only person in finance, and everybody else reports upwards.
var (
	alice   = user.User{ID: "u_alice", Name: "Alice", Role: user.RoleEmployee}
	bob     = user.User{ID: "u_bob", Name: "Bob", Role: user.RoleEmployee}
	carol   = user.User{ID: "u_carol", Name: "Carol", Role: user.RoleManager}
	mallory = user.User{ID: "u_mallory", Name: "Mallory", Role: user.RoleManager}
	peggy   = user.User{ID: "u_peggy", Name: "Peggy", Role: user.RoleManager}
	trent   = user.User{ID: "u_trent", Name: "Trent", Role: user.RoleFinance}
)

var managerOf = map[user.ID]user.User{
	alice.ID:   carol,
	bob.ID:     mallory,
	carol.ID:   peggy,
	mallory.ID: peggy,
	trent.ID:   peggy,
	// Peggy has nobody above her.
}

// subjectOf builds what the service would hand to Route: the values, the
// requester, and the people already looked up in the directory.
func subjectOf(requester user.User, amountCents int64) model.Subject {
	s := model.Subject{
		Values: model.Values{
			Type:        model.TypeTravel,
			AmountCents: amountCents,
			Description: "Customer onsite in Chicago",
		},
		Requester: requester,
		Finance:   &trent,
	}

	if mgr, ok := managerOf[requester.ID]; ok {
		s.Manager = &mgr
	}
	return s
}

type wantStep struct {
	name     string
	approver user.ID
}

func expectChain(chain approval.Chain, want ...wantStep) {
	GinkgoHelper()

	Expect(chain).To(HaveLen(len(want)))
	for i, w := range want {
		Expect(chain[i].Name).To(Equal(w.name), "step %d name", i)
		Expect(chain[i].ApproverID).To(Equal(approval.Approver(w.approver)),
			"step %d approver", i)
		Expect(chain[i].Status).To(Equal(approval.StepStatusPending), "step %d status", i)
	}
}

// ---------------------------------------------------------------------------

var _ = Describe("Route", func() {
	var (
		spec    approval.Spec[model.Subject]
		subject model.Subject
		chain   approval.Chain
		err     error
	)

	JustBeforeEach(func() {
		chain, err = model.Route(spec, subject)
	})

	Describe("the single-step policy the exercise specifies", func() {
		BeforeEach(func() { spec = model.SingleStepSpec() })

		Context("when somebody with a manager claims less than $1,000", func() {
			BeforeEach(func() { subject = subjectOf(alice, 45_000) })

			It("goes to their manager", func() {
				Expect(err).NotTo(HaveOccurred())
				expectChain(chain, wantStep{model.RoleManager, carol.ID})
			})
		})

		// The boundary is the rule, so both sides of it are pinned.
		Context("when the amount is a penny below the threshold", func() {
			BeforeEach(func() { subject = subjectOf(alice, model.ThresholdCents-1) })

			It("still goes to their manager", func() {
				expectChain(chain, wantStep{model.RoleManager, carol.ID})
			})
		})

		Context("when the amount is exactly the threshold", func() {
			BeforeEach(func() { subject = subjectOf(alice, model.ThresholdCents) })

			It("goes to finance instead", func() {
				expectChain(chain, wantStep{model.RoleFinance, trent.ID})
			})
		})

		Context("when the requester has nobody above them", func() {
			BeforeEach(func() { subject = subjectOf(peggy, 45_000) })

			It("falls back to finance, and says so", func() {
				Expect(err).NotTo(HaveOccurred())
				expectChain(chain, wantStep{model.RoleFinance, trent.ID})
			})
		})

		// The fallback only fires when the primary resolution fails, so a
		// finance user's small expense still goes to their own manager.
		Context("when finance claims less than $1,000", func() {
			BeforeEach(func() { subject = subjectOf(trent, 45_000) })

			It("goes to their manager", func() {
				expectChain(chain, wantStep{model.RoleManager, peggy.ID})
			})
		})

		Context("when finance claims $1,000 or more", func() {
			BeforeEach(func() { subject = subjectOf(trent, 150_000) })

			It("is refused, because the only approver would be the requester", func() {
				Expect(err).To(MatchError(approval.ErrNoEligibleApprover))
				Expect(chain).To(BeNil())
			})
		})

		Context("when finance has nobody above them either", func() {
			BeforeEach(func() {
				subject = subjectOf(trent, 45_000)
				subject.Manager = nil
			})

			It("is refused, because the fallback is also the requester", func() {
				Expect(err).To(MatchError(approval.ErrNoEligibleApprover))
			})
		})

		Context("when the organisation has nobody in finance at all", func() {
			BeforeEach(func() {
				subject = subjectOf(peggy, 45_000)
				subject.Finance = nil
			})

			It("is refused", func() {
				Expect(err).To(MatchError(approval.ErrNoEligibleApprover))
			})
		})
	})

	// Multi-step is the optional extension, and the only difference is that the
	// manager rule fires always rather than only below the threshold.
	Describe("the multi-step policy", func() {
		BeforeEach(func() { spec = model.MultiStepSpec() })

		Context("when the amount is below the threshold", func() {
			BeforeEach(func() { subject = subjectOf(alice, 45_000) })

			It("behaves exactly like the single-step policy", func() {
				expectChain(chain, wantStep{model.RoleManager, carol.ID})
			})
		})

		Context("when the amount reaches the threshold", func() {
			BeforeEach(func() { subject = subjectOf(alice, model.ThresholdCents) })

			It("goes to the manager first and then to finance", func() {
				Expect(err).NotTo(HaveOccurred())
				expectChain(chain,
					wantStep{model.RoleManager, carol.ID},
					wantStep{model.RoleFinance, trent.ID},
				)
			})
		})

		Context("when somebody else large claims", func() {
			BeforeEach(func() { subject = subjectOf(bob, 150_000) })

			It("uses their own manager, then the same finance approver", func() {
				expectChain(chain,
					wantStep{model.RoleManager, mallory.ID},
					wantStep{model.RoleFinance, trent.ID},
				)
			})
		})

		// Both stages resolve to Trent — the manager stage by falling back —
		// and asking one person twice in a row is not an approval step.
		Context("when the requester has no manager", func() {
			BeforeEach(func() { subject = subjectOf(peggy, 150_000) })

			It("collapses the two stages into one", func() {
				Expect(err).NotTo(HaveOccurred())
				expectChain(chain, wantStep{model.RoleFinance, trent.ID})
			})
		})

		// The finance stage is required, so it cannot quietly drop out. Leaving
		// the manager stage to stand alone would let the one person finance is
		// meant to check approve their own large expense through their manager.
		Context("when finance makes a large claim", func() {
			BeforeEach(func() { subject = subjectOf(trent, 150_000) })

			It("is refused, even though the manager stage could be staffed", func() {
				Expect(err).To(MatchError(approval.ErrNoEligibleApprover))
				Expect(chain).To(BeNil())
			})
		})

		Context("when finance makes a small claim", func() {
			BeforeEach(func() { subject = subjectOf(trent, 45_000) })

			It("goes to their manager, because the finance stage never fires", func() {
				Expect(err).NotTo(HaveOccurred())
				expectChain(chain, wantStep{model.RoleManager, peggy.ID})
			})
		})

		Context("when finance makes a large claim and has no manager", func() {
			BeforeEach(func() {
				subject = subjectOf(trent, 150_000)
				subject.Manager = nil
			})

			It("is refused, because nothing is left to staff it", func() {
				Expect(err).To(MatchError(approval.ErrNoEligibleApprover))
			})
		})
	})
})
