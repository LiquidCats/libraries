package observer_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/LiquidCats/libraries/observer"
)

type fakeObserver struct {
	mu          sync.Mutex
	data        []any
	wg          *sync.WaitGroup
	returnError bool
}

func (f *fakeObserver) Update(_ context.Context, data any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data = append(f.data, data)
	if f.wg != nil {
		f.wg.Done()
	}
	if f.returnError {
		return errors.New("update error")
	}
	return nil
}

// TestSubjectNotifySingleObserver verifies that a single observer receives the notification.
func TestSubjectNotifySingleObserver(t *testing.T) {
	subj := observer.NewSubject(1)
	eventName := observer.EventName("test")

	wg := &sync.WaitGroup{}
	wg.Add(1)
	fo := &fakeObserver{wg: wg}
	subj.Register(eventName, fo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	var runErr error
	go func() {
		runErr = subj.Run(ctx)
		close(done)
	}()

	// Allow workers to start
	time.Sleep(100 * time.Millisecond)

	subj.Notify(&observer.Event{
		Name: eventName,
		Data: "hello",
	})

	waitCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
		// success
	case <-time.After(1 * time.Second):
		t.Fatal("Observer did not update in time")
	}

	cancel()
	<-done

	fo.mu.Lock()
	defer fo.mu.Unlock()
	if len(fo.data) != 1 || fo.data[0] != "hello" {
		t.Fatalf("Expected observer data [hello], got %v", fo.data)
	}

	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		t.Fatalf("Unexpected error from Run: %v", runErr)
	}
}

// TestSubjectNotifyMultipleObservers verifies that multiple observers registered for the same event are all notified.
func TestSubjectNotifyMultipleObservers(t *testing.T) {
	subj := observer.NewSubject(2)
	eventName := observer.EventName("multi")

	count := 3
	wg := &sync.WaitGroup{}
	wg.Add(count)
	observers := make([]*fakeObserver, count)
	for i := range count {
		fo := &fakeObserver{wg: wg}
		observers[i] = fo
		subj.Register(eventName, fo)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = subj.Run(ctx)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	subj.Notify(&observer.Event{
		Name: eventName,
		Data: 42,
	})

	waitCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
		// success
	case <-time.After(1 * time.Second):
		t.Fatal("Not all observers updated in time")
	}

	cancel()
	<-done

	for i, fo := range observers {
		fo.mu.Lock()
		if len(fo.data) != 1 || fo.data[0] != 42 {
			t.Errorf("Observer %d expected data [42], got %v", i, fo.data)
		}
		fo.mu.Unlock()
	}
}

// TestObserverUpdateError verifies that if an observer returns an error, Run returns that error.
func TestObserverUpdateError(t *testing.T) {
	subj := observer.NewSubject(1)
	eventName := observer.EventName("error")

	wg := &sync.WaitGroup{}
	wg.Add(1)
	fo := &fakeObserver{
		wg:          wg,
		returnError: true,
	}
	subj.Register(eventName, fo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	var runErr error
	go func() {
		runErr = subj.Run(ctx)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	subj.Notify(&observer.Event{
		Name: eventName,
		Data: "error data",
	})

	waitCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
		// success
	case <-time.After(1 * time.Second):
		t.Fatal("Observer update error not received in time")
	}

	cancel()
	<-done

	if runErr == nil {
		t.Fatal("Expected error from Run due to update error, got nil")
	}
}
