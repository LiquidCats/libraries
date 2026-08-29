package observer

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"
)

type EventName string

type Event struct {
	Name EventName
	Data any
}

type Observer interface {
	Update(ctx context.Context, data any) error
}

type Subject struct {
	eventCh      chan *Event
	workersCount int
	workers      map[EventName][]Observer
	mu           sync.RWMutex
}

func NewSubject(workersCount int) *Subject {
	return &Subject{
		eventCh:      make(chan *Event),
		workers:      make(map[EventName][]Observer),
		workersCount: workersCount,
	}
}

func (o *Subject) worker(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-o.eventCh:
			if !ok {
				return nil // channel closed
			}

			o.mu.RLock()
			handlers, exists := o.workers[event.Name]
			o.mu.RUnlock()

			if !exists || len(handlers) == 0 {
				continue
			}

			group, ctx := errgroup.WithContext(ctx)

			for _, h := range handlers {
				handler := h

				group.Go(func() error {
					return handler.Update(ctx, event.Data)
				})
			}

			if err := group.Wait(); err != nil {
				return err
			}
		}
	}
}

func (o *Subject) Run(ctx context.Context) error {
	defer close(o.eventCh)

	group, ctx := errgroup.WithContext(ctx)

	for i := 0; i < o.workersCount; i++ {
		group.Go(func() error {
			return o.worker(ctx)
		})
	}

	return group.Wait()
}

func (o *Subject) Register(event EventName, observer Observer) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, ok := o.workers[event]; !ok {
		o.workers[event] = []Observer{}
	}

	o.workers[event] = append(o.workers[event], observer)
}

func (o *Subject) Notify(event *Event) {
	o.eventCh <- event
}
