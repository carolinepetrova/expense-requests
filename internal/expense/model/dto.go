package model

import (
	"github.com/carolinepetrova/expense-requests/internal/user"
)

type ValuesInput struct {
	ExpenseType             string  `json:"expenseType"`
	AmountCents             int64   `json:"amountCents"`
	Description             string  `json:"description"`
	Billable                bool    `json:"billable"`
	Client                  *string `json:"client"`
	AdditionalJustification *string `json:"additionalJustification"`
	OtherReason             *string `json:"otherReason"`
}

func (in ValuesInput) ToModel() Values {
	return Values{
		Type:                    Type(in.ExpenseType),
		AmountCents:             in.AmountCents,
		Description:             in.Description,
		Billable:                in.Billable,
		Client:                  in.Client,
		AdditionalJustification: in.AdditionalJustification,
		OtherReason:             in.OtherReason,
	}
}

type CreateRequestInput struct {
	Values ValuesInput `json:"values"`
}

type UpdateValuesInput struct {
	ID     ID          `param:"id"`
	Values ValuesInput `json:"values"`
}

type SubmitRequestInput struct {
	ID ID `param:"id"`
}

type DecisionInput struct {
	ID      ID     `param:"id"`
	Comment string `json:"comment"`
}

type GetRequestInput struct {
	ID ID `param:"id"`
}

// ListRequestsInput is the filter bar.
//
// Scope is a word rather than a user id, so nobody can ask for somebody else's
// queue by editing a query string — it is resolved against the caller.
type ListRequestsInput struct {
	Status string `query:"status"`
	Scope  string `query:"scope"`
	Query  string `query:"q"`
}

// UserResponse is the picker's payload. It stands in for a login screen.
type UserResponse struct {
	ID        user.ID  `json:"id"`
	Name      string   `json:"name"`
	Role      string   `json:"role"`
	ManagerID *user.ID `json:"managerId"`
}
