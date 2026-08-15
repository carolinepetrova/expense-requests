package approval_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/carolinepetrova/expense-requests/internal/approval"
)

type subject struct {
	amount    int64
	requester approval.Approver
	manager   approval.Approver
	finance   approval.Approver
	cfo       approval.Approver
}

const threshold int64 = 100_000

func resolve(pick func(subject) approval.Approver) func(subject) (approval.Approver, bool) {
	return func(s subject) (approval.Approver, bool) {
		id := pick(s)
		return id, id != ""
	}
}

var (
	manager = resolve(func(s subject) approval.Approver { return s.manager })
	finance = resolve(func(s subject) approval.Approver { return s.finance })
	cfo     = resolve(func(s subject) approval.Approver { return s.cfo })
)

func atLeast(n int64) func(subject) bool {
	return func(s subject) bool { return s.amount >= n }
}

func under(n int64) func(subject) bool {
	return func(s subject) bool { return s.amount < n }
}

func always(subject) bool { return true }

func specOf(rules ...approval.Rule[subject]) approval.Spec[subject] {
	return approval.Spec[subject]{
		Rules:        rules,
		Fallback:     finance,
		FallbackName: "Finance",
		Requester:    func(s subject) approval.Approver { return s.requester },
	}
}

type wantStep struct {
	name     string
	approver approval.Approver
}

func expectChain(chain approval.Chain, want ...wantStep) {
	GinkgoHelper()

	Expect(chain).To(HaveLen(len(want)))
	for i, w := range want {
		Expect(chain[i].Name).To(Equal(w.name), "step %d name", i)
		Expect(chain[i].ApproverID).To(Equal(w.approver), "step %d approver", i)
		Expect(chain[i].Status).To(Equal(approval.StepStatusPending), "step %d status", i)
	}
}

var _ = Describe("Compile", func() {
	var (
		spec  approval.Spec[subject]
		subj  subject
		chain approval.Chain
		err   error
	)

	JustBeforeEach(func() {
		chain, err = approval.Compile(spec, subj)
	})

	Describe("a spec whose rules are mutually exclusive", func() {
		BeforeEach(func() {
			spec = specOf(
				approval.Rule[subject]{Name: "Manager", When: under(threshold), Who: manager},
				approval.Rule[subject]{Name: "Finance", When: atLeast(threshold), Who: finance},
			)
			subj = subject{requester: "alice", manager: "carol", finance: "trent"}
		})

		Context("when the amount is under the threshold", func() {
			BeforeEach(func() { subj.amount = 45_000 })

			It("routes to the requester's manager", func() {
				Expect(err).NotTo(HaveOccurred())
				expectChain(chain, wantStep{"Manager", "carol"})
			})
		})

		Context("when the amount is exactly the threshold", func() {
			BeforeEach(func() { subj.amount = threshold })

			It("routes to finance", func() {
				Expect(err).NotTo(HaveOccurred())
				expectChain(chain, wantStep{"Finance", "trent"})
			})
		})

		Context("when the amount is above the threshold", func() {
			BeforeEach(func() { subj.amount = 125_000 })

			It("routes to finance", func() {
				Expect(err).NotTo(HaveOccurred())
				expectChain(chain, wantStep{"Finance", "trent"})
			})
		})

		Context("when the requester has no manager", func() {
			BeforeEach(func() {
				subj.amount = 45_000
				subj.manager = ""
			})

			It("falls back to finance", func() {
				Expect(err).NotTo(HaveOccurred())
				expectChain(chain, wantStep{"Finance", "trent"})
			})

			It("names the step after who actually acts, not the rule", func() {
				Expect(chain[0].Name).To(Equal("Finance"))
			})
		})

		Context("when the requester is their own manager", func() {
			BeforeEach(func() {
				subj.amount = 45_000
				subj.requester = "carol"
			})

			It("falls back to finance", func() {
				Expect(err).NotTo(HaveOccurred())
				expectChain(chain, wantStep{"Finance", "trent"})
			})
		})

		// The fallback only fires when the primary resolution fails, so a
		// finance user's small expense still goes to their own manager.
		Context("when finance submits an expense under the threshold", func() {
			BeforeEach(func() {
				subj.amount = 45_000
				subj.requester = "trent"
				subj.manager = "dana"
			})

			It("routes to their manager", func() {
				Expect(err).NotTo(HaveOccurred())
				expectChain(chain, wantStep{"Manager", "dana"})
			})
		})

		Context("when finance submits an expense above the threshold", func() {
			BeforeEach(func() {
				subj.amount = 125_000
				subj.requester = "trent"
				subj.manager = "dana"
			})

			It("refuses, because the only approver would be the requester", func() {
				Expect(err).To(MatchError(approval.ErrNoEligibleApprover))
				Expect(chain).To(BeNil())
			})
		})

		Context("when finance has no manager and submits under the threshold", func() {
			BeforeEach(func() {
				subj.amount = 45_000
				subj.requester = "trent"
				subj.manager = ""
			})

			It("refuses, because the fallback is also the requester", func() {
				Expect(err).To(MatchError(approval.ErrNoEligibleApprover))
			})
		})

		Context("when nobody can be resolved at all", func() {
			BeforeEach(func() {
				subj = subject{amount: 45_000, requester: "alice"}
			})

			It("refuses", func() {
				Expect(err).To(MatchError(approval.ErrNoEligibleApprover))
			})
		})
	})

	Describe("a spec whose rules can fire together", func() {
		BeforeEach(func() {
			spec = specOf(
				approval.Rule[subject]{Name: "Manager", When: always, Who: manager},
				approval.Rule[subject]{Name: "Finance", When: atLeast(threshold), Who: finance},
				approval.Rule[subject]{Name: "CFO", When: atLeast(1_000_000), Who: cfo},
			)
			subj = subject{requester: "bob", manager: "dana", finance: "trent", cfo: "zoe"}
		})

		Context("when some rules fire", func() {
			BeforeEach(func() { subj.amount = 150_000 })

			It("includes only those rules, in declaration order", func() {
				Expect(err).NotTo(HaveOccurred())
				expectChain(chain,
					wantStep{"Manager", "dana"},
					wantStep{"Finance", "trent"},
				)
			})
		})

		Context("when every rule fires", func() {
			BeforeEach(func() { subj.amount = 2_000_000 })

			It("produces the full chain", func() {
				Expect(err).NotTo(HaveOccurred())
				expectChain(chain,
					wantStep{"Manager", "dana"},
					wantStep{"Finance", "trent"},
					wantStep{"CFO", "zoe"},
				)
			})
		})

		Context("when two consecutive stages resolve to the same person", func() {
			BeforeEach(func() {
				subj.amount = 150_000
				subj.manager = "trent"
			})

			It("collapses them into one step", func() {
				Expect(err).NotTo(HaveOccurred())
				expectChain(chain, wantStep{"Manager", "trent"})
			})
		})

		Context("when early stages cannot be staffed by anyone but the requester", func() {
			BeforeEach(func() {
				subj.requester = "trent"
				subj.manager = ""
			})

			Context("and a later stage can be staffed", func() {
				BeforeEach(func() { subj.amount = 2_000_000 })

				It("drops the unstaffable stages and keeps the rest", func() {
					Expect(err).NotTo(HaveOccurred())
					expectChain(chain, wantStep{"CFO", "zoe"})
				})
			})

			Context("and no later stage can be staffed", func() {
				BeforeEach(func() { subj.amount = 150_000 })

				It("refuses", func() {
					Expect(err).To(MatchError(approval.ErrNoEligibleApprover))
				})
			})
		})
	})
})

var _ = Describe("Chain", func() {
	var (
		now      time.Time
		original approval.Chain
	)

	BeforeEach(func() {
		now = time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)

		var err error
		original, err = approval.Compile(
			specOf(
				approval.Rule[subject]{Name: "Manager", When: always, Who: manager},
				approval.Rule[subject]{Name: "Finance", When: always, Who: finance},
			),
			subject{amount: 150_000, requester: "bob", manager: "dana", finance: "trent"},
		)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("a freshly compiled chain", func() {
		It("waits on the first step", func() {
			i, step, ok := original.Current()
			Expect(ok).To(BeTrue())
			Expect(i).To(Equal(0))
			Expect(step.ApproverID).To(Equal(approval.Approver("dana")))
		})

		It("is neither complete nor rejected", func() {
			Expect(original.Complete()).To(BeFalse())
			Expect(original.Rejected()).To(BeFalse())
		})

		It("treats only the first approver as current", func() {
			Expect(original.IsCurrentApprover("dana")).To(BeTrue())
			Expect(original.IsCurrentApprover("trent")).To(BeFalse())
		})
	})

	Describe("approving", func() {
		var (
			chain     approval.Chain
			stepIndex int
			actor     approval.Approver
			comment   string

			result approval.Chain
			err    error
		)

		BeforeEach(func() {
			chain = original
			stepIndex, actor, comment = 0, "dana", "fine by me"
		})

		JustBeforeEach(func() {
			result, err = chain.Approve(stepIndex, actor, comment, now)
		})

		Context("when the current approver approves", func() {
			It("records the decision on that step", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result[0].Status).To(Equal(approval.StepStatusApproved))
				Expect(result[0].Comment).To(Equal("fine by me"))
				Expect(result[0].ActedAt).To(HaveValue(Equal(now)))
			})

			It("advances to the next step without completing the chain", func() {
				Expect(result.Complete()).To(BeFalse())

				i, step, ok := result.Current()
				Expect(ok).To(BeTrue())
				Expect(i).To(Equal(1))
				Expect(step.ApproverID).To(Equal(approval.Approver("trent")))
			})

			It("leaves the original chain untouched", func() {
				Expect(original[0].Status).To(Equal(approval.StepStatusPending))
				Expect(original[0].ActedAt).To(BeNil())
			})
		})

		Context("when a later approver tries to act out of turn", func() {
			BeforeEach(func() { stepIndex, actor = 1, "trent" })

			It("refuses", func() {
				Expect(err).To(MatchError(approval.ErrNotCurrentStep))
				Expect(result).To(BeNil())
			})
		})

		Context("when someone who is not the approver acts", func() {
			BeforeEach(func() { actor = "mallory" })

			It("refuses", func() {
				Expect(err).To(MatchError(approval.ErrNotTheApprover))
			})
		})

		Context("when the first step has already been approved", func() {
			BeforeEach(func() {
				var approveErr error
				chain, approveErr = chain.Approve(0, "dana", "", now)
				Expect(approveErr).NotTo(HaveOccurred())
			})

			Context("and the same person approves again", func() {
				It("refuses, because the chain has moved past them", func() {
					Expect(err).To(MatchError(approval.ErrNotCurrentStep))
				})
			})

			Context("and the last approver approves", func() {
				BeforeEach(func() { stepIndex, actor = 1, "trent" })

				It("completes the chain", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Complete()).To(BeTrue())

					_, _, ok := result.Current()
					Expect(ok).To(BeFalse())
				})
			})
		})
	})

	Describe("rejecting", func() {
		var (
			result approval.Chain
			err    error
		)

		JustBeforeEach(func() {
			result, err = original.Reject(0, "dana", "not this quarter", now)
		})

		It("ends the chain", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Rejected()).To(BeTrue())
			Expect(result.Complete()).To(BeFalse())
		})

		It("records the reason", func() {
			Expect(result[0].Comment).To(Equal("not this quarter"))
		})

		// Later steps were never reached rather than skipped; the host's own
		// status already records why the chain stopped.
		It("leaves unreached steps pending", func() {
			Expect(result[1].Status).To(Equal(approval.StepStatusPending))
		})

		// Those pending steps must not make the chain look like it is still
		// waiting on somebody, or a dead request turns up in their queue.
		It("is waiting on nobody", func() {
			_, _, pending := result.Current()
			Expect(pending).To(BeFalse())
			Expect(result.IsCurrentApprover("trent")).To(BeFalse())
		})

		It("accepts no further decisions", func() {
			_, err := result.Approve(1, "trent", "", now)
			Expect(err).To(MatchError(approval.ErrNotCurrentStep))
		})
	})
})
