package store

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/carolinepetrova/expense-requests/internal/expense"
	"github.com/carolinepetrova/expense-requests/internal/expense/model"
)

// Requests is the in-memory store.
type Requests struct {
	mu      sync.RWMutex
	records map[model.ID]model.Record
	views   map[model.ID]expense.RequestView
	order   []model.ID
}

func NewRequests(seed []model.Record) *Requests {
	s := &Requests{
		records: make(map[model.ID]model.Record, len(seed)),
		views:   make(map[model.ID]expense.RequestView, len(seed)),
		order:   make([]model.ID, 0, len(seed)),
	}

	for _, rec := range seed {
		if _, seen := s.records[rec.ID]; seen {
			continue
		}

		rec.Events = slices.Clone(rec.Events)
		rec.Version = 1

		s.records[rec.ID] = rec
		s.order = append(s.order, rec.ID)

		s.views[rec.ID] = expense.Rehydrate(rec).View()
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
	s.views[r.ID] = r.View()

	return nil
}

func (s *Requests) View(_ context.Context, id model.ID) (expense.RequestView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	view, ok := s.views[id]
	if !ok {
		return expense.RequestView{}, fmt.Errorf("%w: %s", model.ErrNotFound, id)
	}
	return view, nil
}

// Views applies the filter to the materialised projections.
func (s *Requests) Views(
	_ context.Context, f expense.Filter,
) ([]expense.RequestSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]expense.RequestSummary, 0, len(s.order))
	for _, id := range s.order {
		if view := s.views[id]; matches(f, view) {
			out = append(out, view.Summary())
		}
	}

	slices.SortStableFunc(out, func(a, b expense.RequestSummary) int {
		return b.UpdatedAt.Compare(a.UpdatedAt)
	})
	return out, nil
}

// matches reports whether a projection satisfies every part of the filter.
// Unset fields match everything, so a zero Filter returns the whole list.
func matches(f expense.Filter, view expense.RequestView) bool {
	return wanted(f.Status, &view.Status) &&
		wanted(f.RequesterID, &view.RequesterID) &&
		wanted(f.ApproverID, view.ApproverID) &&
		(f.Query == "" || containsFold(view.Values.Description, f.Query))
}

// wanted compares an optional filter value against the projection. An unset
// filter matches anything; a set one needs a value that is both present and
// equal, which is what makes `?approver=u_carol` skip the drafts that are
// waiting on nobody.
func wanted[V comparable](want, got *V) bool {
	return want == nil || (got != nil && *got == *want)
}

func containsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}
