package repository

import (
	"context"
	"reflect"
	"sync"

	"corelog/internal/domain"
)

type Repository struct {
	mu    sync.RWMutex
	store *Store
}

func New(path string) (*Repository, error) {
	store, err := Open(path)
	if err != nil {
		return nil, err
	}
	return &Repository{store: store}, nil
}

func (r *Repository) Snapshot() domain.State {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.store.State()
}

func (r *Repository) Sequence() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.store.Sequence()
}

func (r *Repository) Commit(next domain.State) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.store.Commit(next)
}

func (r *Repository) Transact(change func(*domain.State) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	current := r.store.State()
	next := current.Clone()
	if err := change(&next); err != nil {
		return err
	}
	if reflect.DeepEqual(current, next) {
		return nil
	}
	return r.store.Commit(next)
}

func (r *Repository) TransactContext(ctx context.Context, change func(*domain.State) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	current := r.store.State()
	next := current.Clone()
	if err := change(&next); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if reflect.DeepEqual(current, next) {
		return nil
	}
	return r.store.Commit(next)
}

func (r *Repository) Path() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.store.Path()
}
