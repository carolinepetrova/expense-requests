package expense_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/carolinepetrova/expense-requests/internal/approval"
	"github.com/carolinepetrova/expense-requests/internal/expense"
	"github.com/carolinepetrova/expense-requests/internal/expense/model"
	"github.com/carolinepetrova/expense-requests/internal/user"
)

var (
	alice   = user.User{ID: "u_alice", Name: "Alice", Role: user.RoleEmployee}
	carol   = user.User{ID: "u_carol", Name: "Carol", Role: user.RoleManager}
	trent   = user.User{ID: "u_trent", Name: "Trent", Role: user.RoleFinance}
	mallory = user.User{ID: "u_mallory", Name: "Mallory", Role: user.RoleManager}
)

const requestID = model.ID("REQ-TEST")

var t0 = time.Date(2026, 8, 14, 9, 0, 0, 0, time.UTC)

func at(hours int) time.Time { return t0.Add(time.Duration(hours) * time.Hour) }

func ptr[T any](v T) *T { return &v }

func travel(amountCents int64) model.Values {
	return model.Values{
		Type:        model.TypeTravel,
		AmountCents: amountCents,
		Description: "Customer onsite in Chicago",
	}
}

// chainOf builds the chain the service would have compiled, naming each step
// the way the routing rules do — not after the person's role, so that a change
// to the step names is caught here too.
func chainOf(approvers ...user.User) approval.Chain {
	chain := make(approval.Chain, 0, len(approvers))
	for _, a := range approvers {
		name := model.RoleManager
		if a.Role == user.RoleFinance {
			name = model.RoleFinance
		}

		chain = append(chain, approval.Step{
			Name:       name,
			ApproverID: approval.Approver(a.ID),
			Status:     approval.StepStatusPending,
		})
	}
	return chain
}

var _ = Describe("A request that gets approved", Ordered, ContinueOnFailure, func() {
	var r *expense.Request

	It("begins as a draft owned by its requester", func() {
		By("Alice creating a request")
		r = expense.New(requestID, &model.CreateRequest{
			Command: model.Command{Actor: alice, At: at(0)},
			Values:  travel(45_000),
		})

		Expect(r.Status()).To(Equal(model.StatusDraft))
		Expect(r.RequesterID).To(Equal(alice.ID))
		Expect(r.CurrentApproverID()).To(BeNil())
		Expect(r.Events).To(HaveLen(1))
		Expect(r.Events[0].Type).To(Equal(model.EventTypeCreated))
		Expect(r.CreatedAt()).To(Equal(at(0)))
	})

	It("belongs to nobody else", func() {
		By("Mallory trying to edit and submit somebody else's draft")
		Expect(r.Update(&model.UpdateValues{
			Command: model.Command{Actor: mallory, At: at(1)},
			ID:      r.ID,
			Values:  travel(1),
		})).To(MatchError(model.ErrForbidden))

		Expect(r.Submit(&model.SubmitRequest{
			Command: model.Command{Actor: mallory, At: at(1)},
			ID:      r.ID,
		}, chainOf(carol))).To(MatchError(model.ErrForbidden))

		Expect(r.Values.AmountCents).To(BeEquivalentTo(45_000))
		Expect(r.Status()).To(Equal(model.StatusDraft))
	})

	It("can still be changed while it is a draft", func() {
		By("Alice correcting the amount")
		Expect(r.Update(&model.UpdateValues{
			Command: model.Command{Actor: alice, At: at(1)},
			ID:      r.ID,
			Values:  travel(150_000),
		})).To(Succeed())

		Expect(r.Values.AmountCents).To(BeEquivalentTo(150_000))
		Expect(r.UpdatedAt()).To(Equal(at(1)))

		// An edit is not a decision. Logging every one would bury the
		// transitions that matter.
		Expect(r.Events).To(HaveLen(1), "an edit is not a transition")
	})

	// Routing refuses before it gets this far, but the aggregate does not take
	// that on trust: a request nobody can approve must not exist.
	It("cannot be submitted with nobody to approve it", func() {
		Expect(r.Submit(&model.SubmitRequest{
			Command: model.Command{Actor: alice, At: at(2)},
			ID:      r.ID,
		}, nil)).To(MatchError(model.ErrInvalidTransition))

		Expect(r.Status()).To(Equal(model.StatusDraft))
	})

	It("becomes Submitted and waits on the first approver", func() {
		By("Alice submitting, with a chain the service compiled")
		Expect(r.Submit(&model.SubmitRequest{
			Command: model.Command{Actor: alice, At: at(2)},
			ID:      r.ID,
		}, chainOf(carol, trent))).To(Succeed())

		Expect(r.Status()).To(Equal(model.StatusSubmitted))
		Expect(r.CurrentApproverID()).To(HaveValue(Equal(carol.ID)))

		// The chain is frozen here: a later change to who reports to whom must
		// not reassign a request that is already in flight.
		submitted := r.Events[len(r.Events)-1]
		Expect(submitted.Steps).To(HaveLen(2))
		Expect(submitted.ApproverID).To(HaveValue(Equal(carol.ID)))
	})

	It("can no longer be edited, even by its owner", func() {
		err := r.Update(&model.UpdateValues{
			Command: model.Command{Actor: alice, At: at(3)},
			ID:      r.ID,
			Values:  travel(1),
		})

		// The state is wrong, not the person — the API answers 409, not 403.
		Expect(err).To(MatchError(model.ErrInvalidTransition))
		Expect(err).NotTo(MatchError(model.ErrForbidden))
		Expect(r.Values.AmountCents).To(BeEquivalentTo(150_000))
	})

	It("only accepts a decision from the person it is waiting on", func() {
		By("a bystander, the requester, and a later approver all trying")
		for _, actor := range []user.User{mallory, alice, trent} {
			Expect(r.Approve(&model.ApproveRequest{
				Command: model.Command{Actor: actor, At: at(3)},
				ID:      r.ID,
			})).To(MatchError(model.ErrForbidden), "%s should not be able to approve", actor.Name)
		}

		Expect(r.Status()).To(Equal(model.StatusSubmitted))
		Expect(r.CurrentApproverID()).To(HaveValue(Equal(carol.ID)))
	})

	It("stays Submitted while an intermediate step clears", func() {
		By("Carol approving the first step")
		Expect(r.Approve(&model.ApproveRequest{
			Command: model.Command{Actor: carol, At: at(3)},
			ID:      r.ID,
			Comment: "Reasonable",
		})).To(Succeed())

		Expect(r.Status()).To(Equal(model.StatusSubmitted),
			"a request is not approved until every step has cleared")

		Expect(r.Events[len(r.Events)-1].Type).To(Equal(model.EventTypeStepApproved))
		Expect(r.Chain()[0].Status).To(Equal(approval.StepStatusApproved))
		Expect(r.Chain()[0].Comment).To(Equal("Reasonable"))
	})

	It("moves to the next approver's queue", func() {
		Expect(r.CurrentApproverID()).To(HaveValue(Equal(trent.ID)))

		By("Carol trying again, now that the chain has moved past her")
		Expect(r.Approve(&model.ApproveRequest{
			Command: model.Command{Actor: carol, At: at(4)},
			ID:      r.ID,
		})).To(MatchError(model.ErrForbidden))
	})

	It("becomes Approved when the last step clears", func() {
		By("Trent approving the final step")
		Expect(r.Approve(&model.ApproveRequest{
			Command: model.Command{Actor: trent, At: at(4)},
			ID:      r.ID,
			Comment: "Approved for payment",
		})).To(Succeed())

		Expect(r.Status()).To(Equal(model.StatusApproved))
		Expect(r.Status().Terminal()).To(BeTrue())
		Expect(r.CurrentApproverID()).To(BeNil())
		Expect(r.UpdatedAt()).To(Equal(at(4)))

		final := r.Events[len(r.Events)-1]
		Expect(final.Type).To(Equal(model.EventTypeApproved))
		Expect(final.StepIndex).To(HaveValue(Equal(1)))
		Expect(final.Comment).To(Equal("Approved for payment"))
		Expect(final.ActorID).To(Equal(trent.ID))
	})

	It("accepts nothing further", func() {
		Expect(r.Update(&model.UpdateValues{
			Command: model.Command{Actor: alice, At: at(9)}, ID: r.ID, Values: travel(1),
		})).To(MatchError(model.ErrInvalidTransition))

		Expect(r.Submit(&model.SubmitRequest{
			Command: model.Command{Actor: alice, At: at(9)}, ID: r.ID,
		}, chainOf(carol))).To(MatchError(model.ErrInvalidTransition))

		Expect(r.Approve(&model.ApproveRequest{
			Command: model.Command{Actor: trent, At: at(9)}, ID: r.ID,
		})).To(MatchError(model.ErrInvalidTransition))

		Expect(r.Reject(&model.RejectRequest{
			Command: model.Command{Actor: trent, At: at(9)}, ID: r.ID,
		})).To(MatchError(model.ErrInvalidTransition))
	})

	It("has a history that accounts for every transition", func() {
		types := make([]model.EventType, 0, len(r.Events))
		for _, e := range r.Events {
			types = append(types, e.Type)
		}

		Expect(types).To(Equal([]model.EventType{
			model.EventTypeCreated,
			model.EventTypeSubmitted,
			model.EventTypeStepApproved,
			model.EventTypeApproved,
		}))
	})

	// Status is never stored, so this is what makes "the status always matches
	// the latest action" structural rather than a rule somebody has to
	// remember. If the fold and the commands ever disagreed, it fails here.
	It("replays from its own events to exactly the same state", func() {
		replayed := expense.Rehydrate(r.Record())

		Expect(replayed.Status()).To(Equal(r.Status()))
		Expect(replayed.Chain()).To(Equal(r.Chain()))
		Expect(replayed.CurrentApproverID()).To(Equal(r.CurrentApproverID()))
		Expect(replayed.CreatedAt()).To(Equal(r.CreatedAt()))
		Expect(replayed.UpdatedAt()).To(Equal(r.UpdatedAt()))
		Expect(replayed.View()).To(Equal(r.View()))

		// A real store persists only the new events. If a replay populated
		// them, the whole history would be written again on the next save.
		Expect(replayed.NewEvents()).To(BeEmpty())
	})
})

var _ = Describe("A request that gets rejected part way along", Ordered, ContinueOnFailure, func() {
	var r *expense.Request

	It("reaches the second approver", func() {
		By("Alice submitting a three-step request and Carol clearing the first")
		r = expense.New(requestID, &model.CreateRequest{
			Command: model.Command{Actor: alice, At: at(0)},
			Values:  travel(150_000),
		})

		Expect(r.Submit(&model.SubmitRequest{
			Command: model.Command{Actor: alice, At: at(1)},
			ID:      r.ID,
		}, chainOf(carol, trent, mallory))).To(Succeed())

		Expect(r.Approve(&model.ApproveRequest{
			Command: model.Command{Actor: carol, At: at(2)},
			ID:      r.ID,
			Comment: "Fine by me",
		})).To(Succeed())

		Expect(r.CurrentApproverID()).To(HaveValue(Equal(trent.ID)))
	})

	It("ends the moment somebody rejects", func() {
		By("Trent rejecting the second step")
		Expect(r.Reject(&model.RejectRequest{
			Command: model.Command{Actor: trent, At: at(3)},
			ID:      r.ID,
			Comment: "Not this quarter",
		})).To(Succeed())

		Expect(r.Status()).To(Equal(model.StatusRejected))
		Expect(r.Status().Terminal()).To(BeTrue())
		Expect(r.Events[len(r.Events)-1].Comment).To(Equal("Not this quarter"))
	})

	It("keeps the approval that had already been given", func() {
		Expect(r.Chain()[0].Status).To(Equal(approval.StepStatusApproved))
		Expect(r.Chain()[0].Comment).To(Equal("Fine by me"))
	})

	// The third approver was never reached rather than skipped; the request's
	// own status already records why the chain stopped.
	It("leaves the steps it never reached pending", func() {
		Expect(r.Chain()[1].Status).To(Equal(approval.StepStatusRejected))
		Expect(r.Chain()[2].Status).To(Equal(approval.StepStatusPending))
		Expect(r.CurrentApproverID()).To(BeNil(),
			"a rejected request is waiting on nobody")
	})

	It("cannot be reopened, by its owner or anybody else", func() {
		Expect(r.Update(&model.UpdateValues{
			Command: model.Command{Actor: alice, At: at(9)}, ID: r.ID, Values: travel(1),
		})).To(MatchError(model.ErrInvalidTransition))

		Expect(r.Approve(&model.ApproveRequest{
			Command: model.Command{Actor: mallory, At: at(9)}, ID: r.ID,
		})).To(MatchError(model.ErrInvalidTransition))
	})

	It("replays from its own events to exactly the same state", func() {
		replayed := expense.Rehydrate(r.Record())

		Expect(replayed.Status()).To(Equal(r.Status()))
		Expect(replayed.Chain()).To(Equal(r.Chain()))
		Expect(replayed.View()).To(Equal(r.View()))
	})
})

// requests.json predates multi-step approval: its submitted events carry an
// approverId and no steps, and its decisions carry no step index. These
// branches exist only to keep that file loadable, and would break silently if
// somebody tidied them away.
var _ = Describe("A request loaded from the sample data", Ordered, ContinueOnFailure, func() {
	var r *expense.Request

	seeded := []model.Event{
		{Type: model.EventTypeCreated, At: at(0), ActorID: alice.ID},
		{
			Type: model.EventTypeSubmitted, At: at(1), ActorID: alice.ID,
			ApproverID: ptr(carol.ID),
		},
	}

	It("reads an approver without steps as a chain of one", func() {
		r = expense.Rehydrate(model.Record{
			ID: requestID, RequesterID: alice.ID, Values: travel(4_200), Events: seeded,
		})

		Expect(r.Status()).To(Equal(model.StatusSubmitted))
		Expect(r.Chain()).To(HaveLen(1))
		Expect(r.Chain()[0].Name).To(Equal(model.RoleApprover))
		Expect(r.CurrentApproverID()).To(HaveValue(Equal(carol.ID)))
	})

	It("can then be acted on like any other request", func() {
		By("Carol approving a request that was seeded rather than created here")
		Expect(r.Approve(&model.ApproveRequest{
			Command: model.Command{Actor: mallory, At: at(2)}, ID: r.ID,
		})).To(MatchError(model.ErrForbidden))

		Expect(r.Approve(&model.ApproveRequest{
			Command: model.Command{Actor: carol, At: at(2)},
			ID:      r.ID,
			Comment: "Looks right",
		})).To(Succeed())

		Expect(r.Status()).To(Equal(model.StatusApproved))
		Expect(r.NewEvents()).To(HaveLen(1),
			"only the decision made here is new; the seeded events are not")
	})

	It("resolves a decision that carries no step index", func() {
		replayed := expense.Rehydrate(model.Record{
			ID: requestID, RequesterID: alice.ID, Values: travel(4_200),
			Events: append(seeded, model.Event{
				Type: model.EventTypeApproved, At: at(2), ActorID: carol.ID,
			}),
		})

		Expect(replayed.Status()).To(Equal(model.StatusApproved))
		Expect(replayed.Chain()[0].Status).To(Equal(approval.StepStatusApproved))
		Expect(replayed.CurrentApproverID()).To(BeNil())
	})
})
