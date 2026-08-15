package expense

import (
	"github.com/carolinepetrova/expense-requests/internal/approval"
	"github.com/carolinepetrova/expense-requests/internal/expense/model"
	"github.com/carolinepetrova/expense-requests/internal/user"
)

// CanEdit reports whether the actor may change the form values. Draft is the
// only editable state, so this is the only condition there is.
func CanEdit(r *Request, actor user.User) bool {
	return r.RequesterID == actor.ID && r.status == model.StatusDraft
}

// CanSubmit reports whether the actor may submit the request for approval.
func CanSubmit(r *Request, actor user.User) bool {
	return r.RequesterID == actor.ID && r.status == model.StatusDraft
}

// CanAct reports whether the actor may approve or reject.
func CanAct(r *Request, actor user.User) bool {
	return r.status == model.StatusSubmitted &&
		r.chain.IsCurrentApprover(approval.Approver(actor.ID))
}
