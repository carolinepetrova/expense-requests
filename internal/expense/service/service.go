package service

import (
	"context"
	"fmt"
	"time"

	"github.com/carolinepetrova/expense-requests/internal/client"
	"github.com/carolinepetrova/expense-requests/internal/expense"
	"github.com/carolinepetrova/expense-requests/internal/expense/model"
	"github.com/carolinepetrova/expense-requests/internal/user"
)

type Store interface {
	LoadRequest(ctx context.Context, id model.ID) (*expense.Request, error)
	SaveRequest(ctx context.Context, r *expense.Request) error

	View(ctx context.Context, id model.ID) (expense.RequestView, error)
	Views(ctx context.Context, f expense.Filter) ([]expense.RequestSummary, error)
}

// Service runs the expense request workflow.
type Service struct {
	store   Store
	users   user.Directory
	clients client.Directory
}

func New(store Store, users user.Directory, clients client.Directory) *Service {
	return &Service{store: store, users: users, clients: clients}
}

// CreateRequest starts a draft.
func (s *Service) CreateRequest(
	ctx context.Context, cmd *model.CreateRequest,
) (expense.RequestView, error) {
	cmd.At = time.Now()

	return s.persist(ctx, expense.New(model.NewID(), cmd))
}

// UpdateValues replaces a draft's values.
func (s *Service) UpdateValues(
	ctx context.Context, cmd *model.UpdateValues,
) (expense.RequestView, error) {
	cmd.At = time.Now()

	r, err := s.store.LoadRequest(ctx, cmd.ID)
	if err != nil {
		return expense.RequestView{}, err
	}
	if err := r.Update(cmd); err != nil {
		return expense.RequestView{}, err
	}
	return s.persist(ctx, r)
}

// SubmitRequest validates the form, works out who must approve, and hands the
// resulting chain to the aggregate.
func (s *Service) SubmitRequest(
	ctx context.Context, cmd *model.SubmitRequest,
) (expense.RequestView, error) {
	cmd.At = time.Now()

	r, err := s.store.LoadRequest(ctx, cmd.ID)
	if err != nil {
		return expense.RequestView{}, err
	}
	if err := r.AuthorizeSubmit(cmd.Actor); err != nil {
		return expense.RequestView{}, err
	}
	if err := s.validate(ctx, r.Values); err != nil {
		return expense.RequestView{}, err
	}

	subject := model.Subject{Values: r.Values, Requester: cmd.Actor}

	if subject.Manager, err = s.users.Manager(ctx, cmd.Actor.ID); err != nil {
		return expense.RequestView{}, err
	}
	if subject.Finance, err = s.users.Finance(ctx); err != nil {
		return expense.RequestView{}, err
	}

	chain, err := model.Route(subject)
	if err != nil {
		return expense.RequestView{}, err
	}

	if err := r.Submit(cmd, chain); err != nil {
		return expense.RequestView{}, err
	}
	return s.persist(ctx, r)
}

// ApproveRequest clears the current step. Whether that completes the request is
// the aggregate's decision, not this layer's.
func (s *Service) ApproveRequest(
	ctx context.Context, cmd *model.ApproveRequest,
) (expense.RequestView, error) {
	cmd.At = time.Now()

	r, err := s.store.LoadRequest(ctx, cmd.ID)
	if err != nil {
		return expense.RequestView{}, err
	}
	if err := r.Approve(cmd); err != nil {
		return expense.RequestView{}, err
	}
	return s.persist(ctx, r)
}

// RejectRequest ends the request.
func (s *Service) RejectRequest(
	ctx context.Context, cmd *model.RejectRequest,
) (expense.RequestView, error) {
	cmd.At = time.Now()

	r, err := s.store.LoadRequest(ctx, cmd.ID)
	if err != nil {
		return expense.RequestView{}, err
	}
	if err := r.Reject(cmd); err != nil {
		return expense.RequestView{}, err
	}
	return s.persist(ctx, r)
}

// Request returns the detail projection.
func (s *Service) Request(ctx context.Context, id model.ID) (expense.RequestView, error) {
	return s.store.View(ctx, id)
}

// Requests returns matching summaries, newest activity first.
func (s *Service) Requests(
	ctx context.Context, f expense.Filter,
) ([]expense.RequestSummary, error) {
	return s.store.Views(ctx, f)
}

// Clients returns the set an expense may be billed to, for the dropdown.
func (s *Service) Clients(ctx context.Context) ([]client.Client, error) {
	return s.clients.List(ctx)
}

// Users returns the people the picker offers. It stands in for a login screen.
func (s *Service) Users(ctx context.Context) ([]user.User, error) {
	return s.users.List(ctx)
}

// persist saves the aggregate and returns its projection.
func (s *Service) persist(
	ctx context.Context, r *expense.Request,
) (expense.RequestView, error) {
	if err := s.store.SaveRequest(ctx, r); err != nil {
		return expense.RequestView{}, err
	}
	return r.View(), nil
}

func (s *Service) validate(ctx context.Context, v model.Values) error {
	errs := model.Validate(v)

	if v.Billable && v.Client != nil && *v.Client != "" {
		found, err := s.clients.Get(ctx, *v.Client)
		if err != nil {
			return fmt.Errorf("check client: %w", err)
		}
		if found == nil {
			errs = append(errs, model.FieldError{
				Field:   "client",
				Code:    model.CodeUnknownClient,
				Message: "That client is not one we bill to.",
			})
		}
	}

	return errs.OrNil()
}
