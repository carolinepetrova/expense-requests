package user

//go:generate go-enum --marshal --values -f=$GOFILE

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var ErrNotFound = errors.New("user not found")

type ID string

// ENUM(employee, manager, finance).
type Role string

type User struct {
	ID        ID
	Name      string
	Role      Role
	ManagerID *ID
}

type Directory interface {
	Get(ctx context.Context, id ID) (User, error)
	List(ctx context.Context) ([]User, error)
	Manager(ctx context.Context, id ID) (*User, error)
	Finance(ctx context.Context) (*User, error)
}

type Memory struct {
	mu    sync.RWMutex
	byID  map[ID]User
	order []ID
}

var _ Directory = (*Memory)(nil)

func NewMemory(users []User) *Memory {
	m := &Memory{
		byID:  make(map[ID]User, len(users)),
		order: make([]ID, 0, len(users)),
	}
	for _, u := range users {
		if _, seen := m.byID[u.ID]; seen {
			continue
		}
		m.byID[u.ID] = u
		m.order = append(m.order, u.ID)
	}
	return m
}

func (m *Memory) Get(_ context.Context, id ID) (User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	u, ok := m.byID[id]
	if !ok {
		return User{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return u, nil
}

func (m *Memory) List(_ context.Context) ([]User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]User, 0, len(m.order))
	for _, id := range m.order {
		out = append(out, m.byID[id])
	}
	return out, nil
}

func (m *Memory) Manager(ctx context.Context, id ID) (*User, error) {
	u, err := m.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if u.ManagerID == nil {
		return nil, nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	mgr, ok := m.byID[*u.ManagerID]
	if !ok {
		return nil, nil
	}
	return &mgr, nil
}

func (m *Memory) Finance(_ context.Context) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, id := range m.order {
		if u := m.byID[id]; u.Role == RoleFinance {
			return &u, nil
		}
	}
	return nil, nil
}
