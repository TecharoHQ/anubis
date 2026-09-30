package actorify

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCanceledCallDoesNotBlockActor(t *testing.T) {
	ctx, cancelActor := context.WithCancel(t.Context())
	defer cancelActor()
	started := make(chan struct{})
	release := make(chan struct{})
	a := New(ctx, func(ctx context.Context, n int) (int, error) {
		if n == 1 {
			close(started)
			<-release
		}
		return n, nil
	})
	callCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { _, err := a.Call(callCtx, 1); done <- err }()
	<-started
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	close(release)
	nextCtx, cancelNext := context.WithTimeout(ctx, time.Second)
	defer cancelNext()
	if n, err := a.Call(nextCtx, 2); err != nil || n != 2 {
		t.Fatalf("next call=%d err=%v", n, err)
	}
}

func TestCallPassesContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	a := New(ctx, func(got context.Context, _ int) (bool, error) { return got == ctx, nil })
	if got, err := a.Call(ctx, 1); err != nil || !got {
		t.Fatalf("context passed=%v err=%v", got, err)
	}
}

func TestCanceledCallWithFullInbox(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	a := New(ctx, func(_ context.Context, n int) (int, error) {
		if n == 1 {
			close(started)
			<-release
		}
		return 0, nil
	})
	a.inbox <- &message[int, int]{ctx: ctx, arg: 1, reply: make(chan reply[int], 1)}
	<-started
	for range cap(a.inbox) {
		a.inbox <- &message[int, int]{ctx: ctx, reply: make(chan reply[int], 1)}
	}
	callCtx, cancelCall := context.WithCancel(ctx)
	cancelCall()
	if _, err := a.Call(callCtx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

func TestCallAfterActorStops(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	a := New(ctx, func(context.Context, int) (int, error) { return 0, nil })
	cancel()
	<-a.done
	if _, err := a.Call(t.Context(), 1); !errors.Is(err, ErrActorDied) {
		t.Fatalf("err=%v", err)
	}
}
