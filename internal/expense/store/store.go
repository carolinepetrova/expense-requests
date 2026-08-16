package store

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/carolinepetrova/expense-requests/internal/expense"
	"github.com/carolinepetrova/expense-requests/internal/expense/model"
	"github.com/carolinepetrova/expense-requests/internal/expense/views"
)

// Requests is the in-memory store.
type Requests struct {
	mu sync.RWMutex

	// records is the write side — the event log the aggregate is rebuilt from.
	// projections is the read side, written in the same locked section, so a
	// reader never sees events that the list does not yet reflect.
	records     map[model.ID]model.Record
	projections map[model.ID]views.RequestView

	order []model.ID
}

func NewRequests(seed []model.Record) *Requests {
	s := &Requests{
		records:     make(map[model.ID]model.Record, len(seed)),
		projections: make(map[model.ID]views.RequestView, len(seed)),
		order:       make([]model.ID, 0, len(seed)),
	}

	for _, rec := range seed {
		if _, seen := s.records[rec.ID]; seen {
			continue
		}

		rec.Events = slices.Clone(rec.Events)
		rec.Version = 1

		s.records[rec.ID] = rec
		s.order = append(s.order, rec.ID)

		s.projections[rec.ID] = expense.Rehydrate(rec).View()
	}
	return s
}

// LoadRequest rebuilds the aggregate. Rehydration belongs to the store, so the
// service never handles a raw event log.
func (s *Requests) LoadRequest(_ context.Context, id model.ID) (*expense.Request, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rec, ok := s.records[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", model.ErrNotFound, id)
	}
	return expense.Rehydrate(rec), nil
}

// SaveRequest persists the events and the projection together.
func (s *Requests) SaveRequest(_ context.Context, r *expense.Request) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored, exists := s.records[r.ID]

	switch {
	case exists && stored.Version != r.Version():
		return fmt.Errorf("%w: %s was changed by somebody else", model.ErrConflict, r.ID)

	case !exists && r.Version() != 0:
		return fmt.Errorf("%w: %s", model.ErrNotFound, r.ID)

	case !exists:
		s.order = append(s.order, r.ID)
	}

	rec := r.Record()
	rec.Version = r.Version() + 1

	s.records[r.ID] = rec
	s.projections[r.ID] = r.View()

	return nil
}

func (s *Requests) View(_ context.Context, id model.ID) (views.RequestView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	view, ok := s.projections[id]
	if !ok {
		return views.RequestView{}, fmt.Errorf("%w: %s", model.ErrNotFound, id)
	}
	return view, nil
}

// Views applies the filter to the materialised projections.
func (s *Requests) Views(
	_ context.Context, f views.Filter,
) ([]views.RequestSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]views.RequestSummary, 0, len(s.order))
	for _, id := range s.order {
		if view := s.projections[id]; f.AppliesTo(view) {
			out = append(out, view.Summary())
		}
	}

	slices.SortStableFunc(out, func(a, b views.RequestSummary) int {
		return b.UpdatedAt.Compare(a.UpdatedAt)
	})
	return out, nil
}
