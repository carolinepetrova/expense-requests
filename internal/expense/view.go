package expense

import (
	"time"

	"github.com/carolinepetrova/expense-requests/internal/approval"
	"github.com/carolinepetrova/expense-requests/internal/expense/model"
	"github.com/carolinepetrova/expense-requests/internal/user"
)

// RequestView is the detail projection, kept up to date by the aggregate as
// events apply and written by the store alongside them.
type RequestView struct {
	ID          model.ID     `json:"id"`
	RequesterID user.ID      `json:"requesterId"`
	Status      model.Status `json:"status"`

	ApproverID *user.ID `json:"approverId"`

	Values   model.Values   `json:"values"`
	Steps    approval.Chain `json:"steps"`
	Timeline []model.Event  `json:"timeline"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// RequestSummary is the list row: a subset of the detail projection, not a
// separate one. In a database this is the same table with fewer columns
// selected, which is why only the view is stored.
type RequestSummary struct {
	ID          model.ID     `json:"id"`
	RequesterID user.ID      `json:"requesterId"`
	Status      model.Status `json:"status"`
	ApproverID  *user.ID     `json:"approverId"`
	ExpenseType model.Type   `json:"expenseType"`
	AmountCents int64        `json:"amountCents"`
	Description string       `json:"description"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

func (v RequestView) Summary() RequestSummary {
	return RequestSummary{
		ID:          v.ID,
		RequesterID: v.RequesterID,
		Status:      v.Status,
		ApproverID:  v.ApproverID,
		ExpenseType: v.Values.Type,
		AmountCents: v.Values.AmountCents,
		Description: v.Values.Description,
		UpdatedAt:   v.UpdatedAt,
	}
}

// Filter is a list query. Every field is optional; a zero Filter matches
// everything.
type Filter struct {
	Status *model.Status

	RequesterID *user.ID
	ApproverID  *user.ID

	Query string
}

// View returns the projection of this request, for the store to persist
// alongside the events.
func (r *Request) View() RequestView {
	return RequestView{
		ID:          r.ID,
		RequesterID: r.RequesterID,
		Status:      r.status,
		ApproverID:  r.CurrentApproverID(),
		Values:      r.Values,
		Steps:       r.Chain(),
		Timeline:    r.Events,
		CreatedAt:   r.createdAt,
		UpdatedAt:   r.updatedAt,
	}
}
