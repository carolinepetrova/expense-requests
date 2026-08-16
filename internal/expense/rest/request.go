package rest

import (
	"github.com/carolinepetrova/expense-requests/internal/expense/model"
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

func (in ValuesInput) toModel() model.Values {
	return model.Values{
		Type:                    model.Type(in.ExpenseType),
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
	ID     model.ID    `param:"id"`
	Values ValuesInput `json:"values"`
}

type SubmitRequestInput struct {
	ID model.ID `param:"id"`
}

type DecisionInput struct {
	ID      model.ID `param:"id"`
	Comment string   `json:"comment"`
}

type GetRequestInput struct {
	ID model.ID `param:"id"`
}

type ListRequestsInput struct {
	Status string `query:"status"`
	Scope  string `query:"scope"`
	Query  string `query:"q"`
}
