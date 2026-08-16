package store_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/carolinepetrova/expense-requests/internal/approval"
	"github.com/carolinepetrova/expense-requests/internal/expense"
	"github.com/carolinepetrova/expense-requests/internal/expense/model"
	"github.com/carolinepetrova/expense-requests/internal/expense/store"
	"github.com/carolinepetrova/expense-requests/internal/user"
)

var (
	alice = user.User{ID: "u_alice", Name: "Alice", Role: user.RoleEmployee}
	carol = user.User{ID: "u_carol", Name: "Carol", Role: user.RoleManager}
	trent = user.User{ID: "u_trent", Name: "Trent", Role: user.RoleFinance}
)

const requestID = model.ID("REQ-TEST")

var t0 = time.Date(2026, 8, 14, 9, 0, 0, 0, time.UTC)

func at(hours int) time.Time { return t0.Add(time.Duration(hours) * time.Hour) }

func values(amountCents int64, description string) model.Values {
	return model.Values{
		Type:        model.TypeTravel,
		AmountCents: amountCents,
		Description: description,
	}
}

func chainOf(approvers ...user.User) approval.Chain {
	chain := make(approval.Chain, 0, len(approvers))
	for _, a := range approvers {
		chain = append(chain, approval.Step{
			Name:       model.RoleManager,
			ApproverID: approval.Approver(a.ID),
			Status:     approval.StepStatusPending,
		})
	}
	return chain
}

// submitted builds a request that is waiting on its first approver, and puts
// it in the store.
func submitted(ctx context.Context, s *store.Requests, approvers ...user.User) {
	GinkgoHelper()

	r := expense.New(requestID, &model.CreateRequest{
		Command: model.Command{Actor: alice, At: at(0)},
		Values:  values(45_000, "Customer onsite in Chicago"),
	})
	Expect(r.Submit(&model.SubmitRequest{
		Command: model.Command{Actor: alice, At: at(1)},
		ID:      r.ID,
	}, chainOf(approvers...))).To(Succeed())

	Expect(s.SaveRequest(ctx, r)).To(Succeed())
}

// ---------------------------------------------------------------------------

var _ = Describe("Optimistic concurrency", func() {
	var (
		ctx context.Context
		s   *store.Requests
	)

	BeforeEach(func() {
		ctx = context.Background()
		s = store.NewRequests(nil)
		submitted(ctx, s, carol, trent)
	})

	// Two callers load the same request, both decide, both try to write. Each
	// aggregate is internally consistent — neither can tell the world moved —
	// so the store is the only place the clash can be noticed.
	Context("when two callers act on the same request at once", func() {
		var first, second error

		BeforeEach(func() {
			By("both loading the same state")
			a, err := s.LoadRequest(ctx, requestID)
			Expect(err).NotTo(HaveOccurred())

			b, err := s.LoadRequest(ctx, requestID)
			Expect(err).NotTo(HaveOccurred())
			Expect(a.Version()).To(Equal(b.Version()))

			By("both approving, against the copy each of them holds")
			Expect(a.Approve(&model.ApproveRequest{
				Command: model.Command{Actor: carol, At: at(2)},
				ID:      requestID,
				Comment: "first",
			})).To(Succeed())

			Expect(b.Approve(&model.ApproveRequest{
				Command: model.Command{Actor: carol, At: at(2)},
				ID:      requestID,
				Comment: "second",
			})).To(Succeed(), "the aggregate cannot see that it is stale")

			first = s.SaveRequest(ctx, a)
			second = s.SaveRequest(ctx, b)
		})

		It("lets the first one through", func() {
			Expect(first).NotTo(HaveOccurred())
		})

		It("refuses the second", func() {
			Expect(second).To(MatchError(model.ErrConflict))
		})

		It("keeps the first decision, not the last write", func() {
			view, err := s.View(ctx, requestID)
			Expect(err).NotTo(HaveOccurred())

			Expect(view.Steps[0].Comment).To(Equal("first"))
			Expect(view.Timeline).To(HaveLen(3),
				"the losing write must not have appended a second decision")
		})
	})

	Context("when a caller acts on state it loaded and saves immediately", func() {
		It("is allowed, and the version moves on", func() {
			r, err := s.LoadRequest(ctx, requestID)
			Expect(err).NotTo(HaveOccurred())
			before := r.Version()

			Expect(r.Approve(&model.ApproveRequest{
				Command: model.Command{Actor: carol, At: at(2)}, ID: requestID,
			})).To(Succeed())
			Expect(s.SaveRequest(ctx, r)).To(Succeed())

			reloaded, err := s.LoadRequest(ctx, requestID)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded.Version()).To(Equal(before + 1))
		})
	})

	// Every write bumps the version, including a draft edit that emits no
	// event — so the guarantee does not depend on the event log growing.
	Context("when two callers edit the same draft", func() {
		BeforeEach(func() {
			s = store.NewRequests(nil)

			draft := expense.New(requestID, &model.CreateRequest{
				Command: model.Command{Actor: alice, At: at(0)},
				Values:  values(45_000, "First"),
			})
			Expect(s.SaveRequest(ctx, draft)).To(Succeed())
		})

		It("refuses the second", func() {
			a, err := s.LoadRequest(ctx, requestID)
			Expect(err).NotTo(HaveOccurred())
			b, err := s.LoadRequest(ctx, requestID)
			Expect(err).NotTo(HaveOccurred())

			Expect(a.Update(&model.UpdateValues{
				Command: model.Command{Actor: alice, At: at(1)},
				ID:      requestID, Values: values(45_000, "Alice's edit"),
			})).To(Succeed())
			Expect(b.Update(&model.UpdateValues{
				Command: model.Command{Actor: alice, At: at(1)},
				ID:      requestID, Values: values(45_000, "The other tab"),
			})).To(Succeed())

			Expect(s.SaveRequest(ctx, a)).To(Succeed())
			Expect(s.SaveRequest(ctx, b)).To(MatchError(model.ErrConflict))

			view, err := s.View(ctx, requestID)
			Expect(err).NotTo(HaveOccurred())
			Expect(view.Values.Description).To(Equal("Alice's edit"))
		})
	})

	Context("when a brand new request claims to have been loaded", func() {
		It("is refused, because there is nothing to have loaded it from", func() {
			seeded := store.NewRequests([]model.Record{{
				ID: "REQ-OTHER", RequesterID: alice.ID, Values: values(1, "x"),
				Events: []model.Event{{
					Type: model.EventTypeCreated, At: at(0), ActorID: alice.ID,
				}},
			}})

			ghost, err := seeded.LoadRequest(ctx, "REQ-OTHER")
			Expect(err).NotTo(HaveOccurred())

			Expect(s.SaveRequest(ctx, ghost)).To(MatchError(model.ErrNotFound))
		})
	})
})

var _ = Describe("Seeded requests", func() {
	var (
		ctx context.Context
		s   *store.Requests
	)

	BeforeEach(func() {
		ctx = context.Background()

		approver := carol.ID
		s = store.NewRequests([]model.Record{
			{
				ID: "REQ-001", RequesterID: alice.ID,
				Values: values(45_000, "Customer onsite in Chicago"),
				Events: []model.Event{
					{Type: model.EventTypeCreated, At: at(0), ActorID: alice.ID},
				},
			},
			{
				ID: "REQ-002", RequesterID: alice.ID,
				Values: values(4_200, "Lunch with prospect"),
				Events: []model.Event{
					{Type: model.EventTypeCreated, At: at(1), ActorID: alice.ID},
					{
						Type: model.EventTypeSubmitted, At: at(2), ActorID: alice.ID,
						ApproverID: &approver,
					},
				},
			},
		})
	})

	// The seed goes through the same projection a live write does, so a broken
	// fold shows up at startup rather than as a subtly wrong list later.
	It("projects the seeded events rather than storing a status", func() {
		draft, err := s.View(ctx, "REQ-001")
		Expect(err).NotTo(HaveOccurred())
		Expect(draft.Status).To(Equal(model.StatusDraft))
		Expect(draft.ApproverID).To(BeNil())

		waiting, err := s.View(ctx, "REQ-002")
		Expect(err).NotTo(HaveOccurred())
		Expect(waiting.Status).To(Equal(model.StatusSubmitted))
		Expect(waiting.ApproverID).To(HaveValue(Equal(carol.ID)))
	})

	It("starts them at a version that can be written against", func() {
		r, err := s.LoadRequest(ctx, "REQ-001")
		Expect(err).NotTo(HaveOccurred())

		Expect(r.Update(&model.UpdateValues{
			Command: model.Command{Actor: alice, At: at(5)},
			ID:      "REQ-001", Values: values(60_000, "Revised"),
		})).To(Succeed())

		Expect(s.SaveRequest(ctx, r)).To(Succeed())
	})
})

var _ = Describe("Views", func() {
	var (
		ctx context.Context
		s   *store.Requests
	)

	BeforeEach(func() {
		ctx = context.Background()
		s = store.NewRequests(nil)
		submitted(ctx, s, carol, trent)
	})

	It("finds a submitted request in its approver's queue", func() {
		id := carol.ID
		rows, err := s.Views(ctx, expense.Filter{ApproverID: &id})

		Expect(err).NotTo(HaveOccurred())
		Expect(rows).To(HaveLen(1))
		Expect(rows[0].ID).To(Equal(requestID))
	})

	// A rejected request is waiting on nobody, even though the steps after the
	// rejection are still pending. It must not sit in their queue.
	It("drops a rejected request out of every queue", func() {
		r, err := s.LoadRequest(ctx, requestID)
		Expect(err).NotTo(HaveOccurred())

		Expect(r.Reject(&model.RejectRequest{
			Command: model.Command{Actor: carol, At: at(2)},
			ID:      requestID, Comment: "Not this quarter",
		})).To(Succeed())
		Expect(s.SaveRequest(ctx, r)).To(Succeed())

		for _, who := range []user.User{carol, trent} {
			id := who.ID
			rows, err := s.Views(ctx, expense.Filter{ApproverID: &id})

			Expect(err).NotTo(HaveOccurred())
			Expect(rows).To(BeEmpty(), "%s should have nothing waiting", who.Name)
		}
	})

	It("filters by status and by a case-insensitive description match", func() {
		status := model.StatusSubmitted
		rows, err := s.Views(ctx, expense.Filter{Status: &status, Query: "CHICAGO"})

		Expect(err).NotTo(HaveOccurred())
		Expect(rows).To(HaveLen(1))

		approved := model.StatusApproved
		rows, err = s.Views(ctx, expense.Filter{Status: &approved})

		Expect(err).NotTo(HaveOccurred())
		Expect(rows).To(BeEmpty())
	})
})
