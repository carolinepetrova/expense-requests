package client

import (
	"context"
	"sync"
)

type Client struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Directory looks clients up.
//
// Context and error are on both methods even though the in-memory
// implementation needs neither: the moment this is a table or an HTTP call it
// will need them, and adding them later touches every caller.
type Directory interface {
	List(ctx context.Context) ([]Client, error)

	// Get returns the client, or nil when no such client exists. An unknown
	// reference is an ordinary answer rather than an error — the caller turns
	// it into a field error on the form.
	Get(ctx context.Context, id string) (*Client, error)
}

// Memory is a Directory backed by a fixed set of clients, loaded at startup.
type Memory struct {
	mu    sync.RWMutex
	byID  map[string]Client
	order []string
}

var _ Directory = (*Memory)(nil)

func NewMemory(clients []Client) *Memory {
	m := &Memory{
		byID:  make(map[string]Client, len(clients)),
		order: make([]string, 0, len(clients)),
	}
	for _, c := range clients {
		if _, seen := m.byID[c.ID]; seen {
			continue
		}
		m.byID[c.ID] = c
		m.order = append(m.order, c.ID)
	}
	return m
}

// List returns clients in seed order, so the dropdown is stable between
// restarts.
func (m *Memory) List(_ context.Context) ([]Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]Client, 0, len(m.order))
	for _, id := range m.order {
		out = append(out, m.byID[id])
	}
	return out, nil
}

func (m *Memory) Get(_ context.Context, id string) (*Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	c, ok := m.byID[id]
	if !ok {
		return nil, nil
	}
	return &c, nil
}
