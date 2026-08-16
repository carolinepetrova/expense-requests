package expense

import (
	"fmt"
	"slices"
	"time"

	"github.com/carolinepetrova/expense-requests/internal/approval"
	"github.com/carolinepetrova/expense-requests/internal/expense/model"
	"github.com/carolinepetrova/expense-requests/internal/expense/views"
	"github.com/carolinepetrova/expense-requests/internal/user"
)

type Request struct {
	ID          model.ID
	RequesterID user.ID
	Values      model.Values
	Events      []model.Event

	status    model.Status
	chain     approval.Chain
	timeline  []views.TimelineEntry
	createdAt time.Time
	updatedAt time.Time

	// version is the version this request was loaded at. A new request is at
	// 0; the store compares it against what is stored before writing.
	version int

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
func Rehydrate(rec model.Record) *Request {
	r := &Request{
		ID:          rec.ID,
		RequesterID: rec.RequesterID,
		Values:      rec.Values,
		Events:      slices.Clone(rec.Events),
		status:      model.StatusDraft,
		version:     rec.Version,
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

func (r *Request) Version() int { return r.version }

func (r *Request) Record() model.Record {
	return model.Record{
		ID:          r.ID,
		RequesterID: r.RequesterID,
		Values:      r.Values,
		Events:      slices.Clone(r.Events),
		Version:     r.version,
	}
}

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

func (r *Request) View() views.RequestView {
	steps := r.Chain()
	if steps == nil {
		steps = approval.Chain{}
	}

	return views.RequestView{
		ID:          r.ID,
		RequesterID: r.RequesterID,
		Status:      r.status,
		ApproverID:  r.CurrentApproverID(),
		Values:      r.Values,
		Steps:       steps,
		Timeline:    slices.Clone(r.timeline),
		CreatedAt:   r.createdAt,
		UpdatedAt:   r.updatedAt,
	}
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

	r.timeline = append(r.timeline, views.TimelineEntry{
		Type:    e.Type,
		At:      e.At,
		ActorID: e.ActorID,
		Comment: e.Comment,
	})
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

// decide stamps an outcome onto a step. Events written by this service always
// carry the step index; the ones in the sample data do not, so the step that
// was current at the time is used instead.
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
