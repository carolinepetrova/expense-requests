package user_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/carolinepetrova/expense-requests/internal/user"
)

func ptr(id user.ID) *user.ID { return &id }

var _ = Describe("Memory directory", func() {
	var (
		ctx   context.Context
		users []user.User
		dir   *user.Memory
	)

	BeforeEach(func() {
		ctx = context.Background()

		// The seed organisation: Peggy is at the top, Trent is the only
		// finance user, everyone else reports upwards.
		users = []user.User{
			{ID: "u_alice", Name: "Alice", Role: user.RoleEmployee, ManagerID: ptr("u_carol")},
			{ID: "u_carol", Name: "Carol", Role: user.RoleManager, ManagerID: ptr("u_peggy")},
			{ID: "u_peggy", Name: "Peggy", Role: user.RoleManager},
			{ID: "u_trent", Name: "Trent", Role: user.RoleFinance, ManagerID: ptr("u_peggy")},
		}
	})

	JustBeforeEach(func() {
		dir = user.NewMemory(users)
	})

	Describe("Get", func() {
		Context("when the user exists", func() {
			It("returns them", func() {
				got, err := dir.Get(ctx, "u_alice")
				Expect(err).NotTo(HaveOccurred())
				Expect(got.Name).To(Equal("Alice"))
				Expect(got.Role).To(Equal(user.RoleEmployee))
			})
		})

		Context("when the user does not exist", func() {
			It("reports not found", func() {
				_, err := dir.Get(ctx, "u_nobody")
				Expect(err).To(MatchError(user.ErrNotFound))
			})
		})
	})

	Describe("List", func() {
		It("preserves seed order, so the picker is stable", func() {
			got, err := dir.List(ctx)
			Expect(err).NotTo(HaveOccurred())

			names := make([]string, 0, len(got))
			for _, u := range got {
				names = append(names, u.Name)
			}
			Expect(names).To(Equal([]string{"Alice", "Carol", "Peggy", "Trent"}))
		})
	})

	Describe("Manager", func() {
		Context("when the user has a manager", func() {
			It("returns them", func() {
				mgr, err := dir.Manager(ctx, "u_alice")
				Expect(err).NotTo(HaveOccurred())
				Expect(mgr).NotTo(BeNil())
				Expect(mgr.ID).To(Equal(user.ID("u_carol")))
			})
		})

		// Somebody has to be at the top of the tree, so this is a normal
		// state rather than an error.
		Context("when the user is at the top of the tree", func() {
			It("reports no manager without failing", func() {
				mgr, err := dir.Manager(ctx, "u_peggy")
				Expect(err).NotTo(HaveOccurred())
				Expect(mgr).To(BeNil())
			})
		})

		Context("when the managerId points at somebody who has left", func() {
			BeforeEach(func() {
				users = append(users, user.User{
					ID: "u_ghost", Name: "Ghost", Role: user.RoleEmployee,
					ManagerID: ptr("u_departed"),
				})
			})

			It("is treated as having no manager, so routing can fall back", func() {
				mgr, err := dir.Manager(ctx, "u_ghost")
				Expect(err).NotTo(HaveOccurred())
				Expect(mgr).To(BeNil())
			})
		})

		Context("when the user does not exist", func() {
			It("reports not found", func() {
				_, err := dir.Manager(ctx, "u_nobody")
				Expect(err).To(MatchError(user.ErrNotFound))
			})
		})
	})

	Describe("Finance", func() {
		Context("when somebody holds the finance role", func() {
			It("returns them", func() {
				fin, err := dir.Finance(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(fin).NotTo(BeNil())
				Expect(fin.ID).To(Equal(user.ID("u_trent")))
			})
		})

		Context("when more than one person holds it", func() {
			BeforeEach(func() {
				users = append(users, user.User{
					ID: "u_frank", Name: "Frank", Role: user.RoleFinance,
				})
			})

			It("picks the first in seed order, so routing is deterministic", func() {
				fin, err := dir.Finance(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(fin).NotTo(BeNil())
				Expect(fin.ID).To(Equal(user.ID("u_trent")))
			})
		})

		Context("when nobody holds it", func() {
			BeforeEach(func() {
				users = []user.User{
					{ID: "u_peggy", Name: "Peggy", Role: user.RoleManager},
				}
			})

			It("reports none without failing", func() {
				fin, err := dir.Finance(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(fin).To(BeNil())
			})
		})
	})
})
