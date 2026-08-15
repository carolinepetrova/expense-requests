package model_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/carolinepetrova/expense-requests/internal/expense/model"
)

func ptr[T any](v T) *T { return &v }

// broken lists the failures as "field:code" pairs, so an assertion reads like
// a row of the rule table rather than a struct comparison.
func broken(errs model.FieldErrors) []string {
	out := make([]string, 0, len(errs))
	for _, e := range errs {
		out = append(out, e.Field+":"+e.Code)
	}
	return out
}

var _ = Describe("Validate", func() {
	var (
		values model.Values
		errs   model.FieldErrors
	)

	// A complete, valid form. Each context below breaks exactly one thing, so
	// what a spec is about is whatever it changes.
	BeforeEach(func() {
		values = model.Values{
			Type:        model.TypeTravel,
			AmountCents: 45_000,
			Description: "Customer onsite in Chicago",
			Billable:    false,
		}
	})

	JustBeforeEach(func() {
		errs = model.Validate(values)
	})

	Context("when every rule is satisfied", func() {
		It("reports nothing", func() {
			Expect(broken(errs)).To(BeEmpty())
		})
	})

	// ------------------------------------------------------- always required

	Context("when the expense type is missing", func() {
		BeforeEach(func() { values.Type = "" })

		It("requires it", func() {
			Expect(broken(errs)).To(ConsistOf("expenseType:required"))
		})
	})

	Context("when the expense type is not one we recognise", func() {
		BeforeEach(func() { values.Type = "Yacht" })

		It("rejects it as invalid rather than missing", func() {
			Expect(broken(errs)).To(ConsistOf("expenseType:invalid"))
		})
	})

	Context("when the description is only whitespace", func() {
		BeforeEach(func() { values.Description = "   " })

		It("requires it", func() {
			Expect(broken(errs)).To(ConsistOf("description:required"))
		})
	})

	Context("when the amount is negative", func() {
		BeforeEach(func() { values.AmountCents = -1 })

		It("rejects it", func() {
			Expect(broken(errs)).To(ConsistOf("amountCents:negative_amount"))
		})
	})

	// A claim for nothing is an abandoned form rather than an expense. The
	// exercise only says "required, can't be negative", so this is an
	// interpretation — see NOTES.
	Context("when the amount is zero", func() {
		BeforeEach(func() { values.AmountCents = 0 })

		It("treats it as not filled in", func() {
			Expect(broken(errs)).To(ConsistOf("amountCents:required"))
		})
	})

	// -------------------------------------------------- the conditional rules

	Describe("the client, which depends on the billable checkbox", func() {
		Context("when the expense is not billable", func() {
			It("does not ask for a client", func() {
				Expect(broken(errs)).To(BeEmpty())
			})
		})

		Context("when the expense is billable", func() {
			BeforeEach(func() { values.Billable = true })

			Context("and no client has been chosen", func() {
				It("requires one", func() {
					Expect(broken(errs)).To(ConsistOf("client:required_when_billable"))
				})
			})

			Context("and the client is blank rather than absent", func() {
				BeforeEach(func() { values.Client = ptr("  ") })

				It("still requires one", func() {
					Expect(broken(errs)).To(ConsistOf("client:required_when_billable"))
				})
			})

			Context("and a client has been chosen", func() {
				BeforeEach(func() { values.Client = ptr("Acme") })

				It("is satisfied", func() {
					Expect(broken(errs)).To(BeEmpty())
				})
			})
		})
	})

	Describe("the justification, which depends on the amount", func() {
		// The boundary is the rule, so both sides of it are pinned here.
		Context("when the amount is a penny below the threshold", func() {
			BeforeEach(func() { values.AmountCents = model.ThresholdCents - 1 })

			It("does not ask for a justification", func() {
				Expect(broken(errs)).To(BeEmpty())
			})
		})

		Context("when the amount is exactly the threshold", func() {
			BeforeEach(func() { values.AmountCents = model.ThresholdCents })

			Context("and there is no justification", func() {
				It("requires one", func() {
					Expect(broken(errs)).To(
						ConsistOf("additionalJustification:required_when_large"))
				})
			})

			Context("and a justification has been given", func() {
				BeforeEach(func() {
					values.AdditionalJustification = ptr("Cheaper than paying monthly")
				})

				It("is satisfied", func() {
					Expect(broken(errs)).To(BeEmpty())
				})
			})
		})
	})

	Describe("the reason, which depends on the expense type", func() {
		Context("when the type is anything but Other", func() {
			It("does not ask for a reason", func() {
				Expect(broken(errs)).To(BeEmpty())
			})
		})

		Context("when the type is Other", func() {
			BeforeEach(func() { values.Type = model.TypeOther })

			Context("and no reason has been given", func() {
				It("requires one", func() {
					Expect(broken(errs)).To(ConsistOf("otherReason:required_when_other"))
				})
			})

			Context("and a reason has been given", func() {
				BeforeEach(func() { values.OtherReason = ptr("Conference sponsorship") })

				It("is satisfied", func() {
					Expect(broken(errs)).To(BeEmpty())
				})
			})
		})
	})

	// ------------------------------------------------------------- reporting

	// The form marks every bad field in one pass, so validation cannot stop at
	// the first failure.
	Context("when several rules are broken at once", func() {
		BeforeEach(func() {
			values = model.Values{
				Type:        model.TypeOther,
				AmountCents: model.ThresholdCents,
				Description: "",
				Billable:    true,
			}
		})

		It("reports all of them", func() {
			Expect(broken(errs)).To(ConsistOf(
				"description:required",
				"client:required_when_billable",
				"additionalJustification:required_when_large",
				"otherReason:required_when_other",
			))
		})
	})

	Context("when a draft is completely empty", func() {
		BeforeEach(func() { values = model.Values{} })

		It("reports the three unconditional rules and nothing else", func() {
			Expect(broken(errs)).To(ConsistOf(
				"expenseType:required",
				"amountCents:required",
				"description:required",
			))
		})
	})
})
