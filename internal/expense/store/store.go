package store

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/carolinepetrova/expense-requests/internal/expense"
	"github.com/carolinepetrova/expense-requests/internal/expense/model"
	"github.com/carolinepetrova/expense-requests/internal/user"
)

// Record is a request as it sits on the write side: mutable values,
// append-only events. Status is absent by design — it is folded on load.
type Record struct {
	ID          model.ID
	RequesterID user.ID
	Values      model.Values
	Events      []model.Event
}

// Requests is the in-memory store.
type Requests struct {
	mu      sync.RWMutex
	records map[model.ID]Record
	views   map[model.ID]expense.RequestView
	order   []model.ID
}

func NewRequests(seed []Record) *Requests {
	s := &Requests{
		records: make(map[model.ID]Record, len(seed)),
		views:   make(map[model.ID]expense.RequestView, len(seed)),
		order:   make([]model.ID, 0, len(seed)),
	}

	for _, rec := range seed {
		if _, seen := s.records[rec.ID]; seen {
			continue
		}
		s.records[rec.ID] = rec
		s.order = append(s.order, rec.ID)

		s.views[rec.ID] = expense.
			Rehydrate(rec.ID, rec.RequesterID, rec.Values, rec.Events).
			View()
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
	return expense.Rehydrate(rec.ID, rec.RequesterID, rec.Values, rec.Events), nil
}

// SaveRequest persists the events and the projection together.
func (s *Requests) SaveRequest(_ context.Context, r *expense.Request) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, seen := s.records[r.ID]; !seen {
		s.order = append(s.order, r.ID)
	}

	s.records[r.ID] = Record{
		ID:          r.ID,
		RequesterID: r.RequesterID,
		Values:      r.Values,
		Events:      slices.Clone(r.Events),
	}
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
		view := s.views[id]

		if f.Status != nil && view.Status != *f.Status {
			continue
		}
		if f.RequesterID != nil && view.RequesterID != *f.RequesterID {
			continue
		}
		if f.ApproverID != nil && (view.ApproverID == nil || *view.ApproverID != *f.ApproverID) {
			continue
		}
		if f.Query != "" && !containsFold(view.Values.Description, f.Query) {
			continue
		}
		out = append(out, view.Summary())
	}

	slices.SortStableFunc(out, func(a, b expense.RequestSummary) int {
		return b.UpdatedAt.Compare(a.UpdatedAt)
	})
	return out, nil
}

func containsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}
