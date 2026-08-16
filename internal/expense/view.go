package expense

import (
	"time"

	"github.com/carolinepetrova/expense-requests/internal/approval"
	"github.com/carolinepetrova/expense-requests/internal/expense/model"
	"github.com/carolinepetrova/expense-requests/internal/user"
)

type TimelineEntry struct {
	Type    model.EventType `json:"type"`
	At      time.Time       `json:"at"`
	ActorID user.ID         `json:"actorId"`
	Comment string          `json:"comment,omitempty"`
}

// RequestView is the detail projection, kept up to date by the aggregate as
// events apply and written by the store alongside them.
type RequestView struct {
	ID          model.ID     `json:"id"`
	RequesterID user.ID      `json:"requesterId"`
	Status      model.Status `json:"status"`

	ApproverID *user.ID `json:"approverId"`

	Values   model.Values    `json:"values"`
	Steps    approval.Chain  `json:"steps"`
	Timeline []TimelineEntry `json:"timeline"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

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

func (r *Request) View() RequestView {
	steps := r.Chain()
	if steps == nil {
		steps = approval.Chain{}
	}

	timeline := make([]TimelineEntry, 0, len(r.Events))
	for _, e := range r.Events {
		timeline = append(timeline, TimelineEntry{
			Type:    e.Type,
			At:      e.At,
			ActorID: e.ActorID,
			Comment: e.Comment,
		})
	}

	return RequestView{
		ID:          r.ID,
		RequesterID: r.RequesterID,
		Status:      r.status,
		ApproverID:  r.CurrentApproverID(),
		Values:      r.Values,
		Steps:       steps,
		Timeline:    timeline,
		CreatedAt:   r.createdAt,
		UpdatedAt:   r.updatedAt,
	}
}
