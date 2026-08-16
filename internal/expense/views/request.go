package views

import (
	"strings"
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

// RequestSummary is the list projection.
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

type Filter struct {
	Status *model.Status

	RequesterID *user.ID
	ApproverID  *user.ID

	Query string
}

func (f Filter) AppliesTo(v RequestView) bool {
	return wanted(f.Status, &v.Status) &&
		wanted(f.RequesterID, &v.RequesterID) &&
		wanted(f.ApproverID, v.ApproverID) &&
		(f.Query == "" || containsFold(v.Values.Description, f.Query))
}

func wanted[V comparable](want, got *V) bool {
	return want == nil || (got != nil && *got == *want)
}

func containsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}
