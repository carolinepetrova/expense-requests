package expense

import (
	"fmt"
	"slices"
	"time"

	"github.com/carolinepetrova/expense-requests/internal/approval"
	"github.com/carolinepetrova/expense-requests/internal/expense/model"
	"github.com/carolinepetrova/expense-requests/internal/user"
)

type Request struct {
	ID          model.ID
	RequesterID user.ID
	Values      model.Values
	Events      []model.Event

	status    model.Status
	chain     approval.Chain
	createdAt time.Time
	updatedAt time.Time

	newEvents []model.Event
}

// New starts a draft.
func New(id model.ID, cmd *model.CreateRequest) *Request {
	r := &Request{
		ID:          id,
		RequesterID: cmd.Actor.ID,
		Values:      cmd.Values,
	}
	r.Apply(model.Event{
		Type: model.EventTypeCreated, At: cmd.At, ActorID: cmd.Actor.ID,
	})
	return r
}

// Rehydrate rebuilds a request from storage. This is the only fold on the
// write path, and there is none at all on the read path — queries go to the
// projections instead.
func Rehydrate(id model.ID, requester user.ID, v model.Values, events []model.Event) *Request {
	r := &Request{
		ID:          id,
		RequesterID: requester,
		Values:      v,
		Events:      slices.Clone(events),
		status:      model.StatusDraft,
	}
	for _, e := range r.Events {
		r.when(e)
	}
	return r
}

// ---------------------------------------------------------------- accessors

func (r *Request) Status() model.Status     { return r.status }
func (r *Request) Chain() approval.Chain    { return slices.Clone(r.chain) }
func (r *Request) CreatedAt() time.Time     { return r.createdAt }
func (r *Request) UpdatedAt() time.Time     { return r.updatedAt }
func (r *Request) NewEvents() []model.Event { return r.newEvents }

// CurrentApproverID is whose decision is outstanding, or nil when none is.
// A convenience for list views that have no interest in the chain.
func (r *Request) CurrentApproverID() *user.ID {
	_, step, ok := r.chain.Current()
	if !ok {
		return nil
	}
	id := user.ID(step.ApproverID)
	return &id
}

func (r *Request) AuthorizeEdit(actor user.User) error {
	if !CanEdit(r, actor) {
		return r.denyChange(actor)
	}
	return nil
}

// AuthorizeSubmit reports why the actor may not submit this request, or nil.
func (r *Request) AuthorizeSubmit(actor user.User) error {
	if !CanSubmit(r, actor) {
		return r.denyChange(actor)
	}
	return nil
}

func (r *Request) Update(cmd *model.UpdateValues) error {
	if err := r.AuthorizeEdit(cmd.Actor); err != nil {
		return err
	}
	r.Values = cmd.Values
	r.updatedAt = cmd.At
	return nil
}

func (r *Request) Submit(cmd *model.SubmitRequest, chain approval.Chain) error {
	if err := r.AuthorizeSubmit(cmd.Actor); err != nil {
		return err
	}
	if len(chain) == 0 {
		return fmt.Errorf("%w: cannot submit without an approval chain",
			model.ErrInvalidTransition)
	}

	approver := user.ID(chain[0].ApproverID)
	r.Apply(model.Event{
		Type:       model.EventTypeSubmitted,
		At:         cmd.At,
		ActorID:    cmd.Actor.ID,
		ApproverID: &approver,
		Steps:      slices.Clone(chain),
	})
	return nil
}

func (r *Request) Approve(cmd *model.ApproveRequest) error {
	i, err := r.act(cmd.Actor)
	if err != nil {
		return err
	}

	next, err := r.chain.Approve(i, approval.Approver(cmd.Actor.ID), cmd.Comment, cmd.At)
	if err != nil {
		return err
	}

	eventType := model.EventTypeStepApproved
	if next.Complete() {
		eventType = model.EventTypeApproved
	}

	r.Apply(model.Event{
		Type: eventType, At: cmd.At, ActorID: cmd.Actor.ID,
		StepIndex: &i, Comment: cmd.Comment,
	})
	return nil
}

func (r *Request) Reject(cmd *model.RejectRequest) error {
	i, err := r.act(cmd.Actor)
	if err != nil {
		return err
	}
	if _, err := r.chain.Reject(
		i, approval.Approver(cmd.Actor.ID), cmd.Comment, cmd.At,
	); err != nil {
		return err
	}

	r.Apply(model.Event{
		Type: model.EventTypeRejected, At: cmd.At, ActorID: cmd.Actor.ID,
		StepIndex: &i, Comment: cmd.Comment,
	})
	return nil
}

func (r *Request) Apply(e model.Event) {
	r.Events = append(r.Events, e)
	r.newEvents = append(r.newEvents, e)
	r.when(e)
}

// when is the fold: the only place status and the chain move. It runs
// identically whether the event has just been created or is being replayed
// from storage, which is what keeps a rehydrated request identical to the one
// that produced the events.
//
// Nothing here validates. These are facts that already happened.
func (r *Request) when(e model.Event) {
	if r.createdAt.IsZero() {
		r.createdAt = e.At
	}
	r.updatedAt = e.At

	switch e.Type {
	case model.EventTypeCreated:
		r.status = model.StatusDraft

	case model.EventTypeSubmitted:
		r.status = model.StatusSubmitted
		r.chain = model.ChainFrom(e)

	case model.EventTypeStepApproved:
		r.decide(e, approval.StepStatusApproved)
		r.status = model.StatusSubmitted

	case model.EventTypeApproved:
		r.decide(e, approval.StepStatusApproved)
		r.status = model.StatusApproved

	case model.EventTypeRejected:
		r.decide(e, approval.StepStatusRejected)
		r.status = model.StatusRejected
	}
}

// act authorises a decision and returns the step it applies to.
func (r *Request) act(actor user.User) (int, error) {
	if r.status != model.StatusSubmitted {
		return 0, fmt.Errorf("%w: cannot act on a %s request",
			model.ErrInvalidTransition, r.status)
	}

	i, step, ok := r.chain.Current()
	if !ok || step.ApproverID != approval.Approver(actor.ID) {
		return 0, fmt.Errorf("%w: not the assigned approver", model.ErrForbidden)
	}
	return i, nil
}

func (r *Request) denyChange(actor user.User) error {
	if r.RequesterID != actor.ID {
		return fmt.Errorf("%w: only the requester may change this request",
			model.ErrForbidden)
	}
	return fmt.Errorf("%w: a %s request cannot be changed",
		model.ErrInvalidTransition, r.status)
}

// decide stamps an outcome onto a step during replay. Events written by this
// package always carry a step index; ones from the sample data do not, so the
// current step is used instead.
func (r *Request) decide(e model.Event, outcome approval.StepStatus) {
	i := -1
	if e.StepIndex != nil {
		i = *e.StepIndex
	} else if idx, _, ok := r.chain.Current(); ok {
		i = idx
	}
	if i < 0 || i >= len(r.chain) {
		return
	}

	at := e.At
	r.chain[i].Status = outcome
	r.chain[i].Comment = e.Comment
	r.chain[i].ActedAt = &at
}
