// Package actorify lets you transform a parallel operation into a serialized
// operation via the Actor pattern[1].
//
// [1]: https://en.wikipedia.org/wiki/Actor_model
package actorify

import (
	"context"
	"errors"
)

func z[Z any]() Z {
	var z Z
	return z
}

var (
	// ErrActorDied is returned when the actor inbox or reply channel was closed.
	ErrActorDied = errors.New("actorify: the actor inbox or reply channel was closed")
)

// Handler is a function alias for the underlying logic the Actor should call.
type Handler[Input, Output any] func(ctx context.Context, input Input) (Output, error)

// Actor is a serializing wrapper that runs a function in a background goroutine.
// Whenever the Call method is invoked, a message is sent to the actor's inbox and then
// the callee waits for a response. Depending on how busy the actor is, this may take
// a moment.
type Actor[Input, Output any] struct {
	handler Handler[Input, Output]
	inbox   chan *message[Input, Output]
}

type message[Input, Output any] struct {
	ctx   context.Context
	arg   Input
	reply chan reply[Output]
}

type reply[Output any] struct {
	output Output
	err    error
}

// New constructs a new Actor and starts its background thread. Cancel the context and you cancel
// the Actor.
func New[Input, Output any](ctx context.Context, handler Handler[Input, Output]) *Actor[Input, Output] {
	result := &Actor[Input, Output]{
		handler: handler,
		inbox:   make(chan *message[Input, Output], 32),
	}

	go result.handle(ctx)

	return result
}

func (a *Actor[Input, Output]) handle(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			close(a.inbox)
			return
		case msg, ok := <-a.inbox:
			if !ok {
				if msg.reply != nil {
					close(msg.reply)
				}

				return
			}

			result, err := a.handler(msg.ctx, msg.arg)

			reply := reply[Output]{
				output: result,
				err:    err,
			}

			msg.reply <- reply
		}
	}
}

// Call calls the Actor with a given Input and returns the handler's Output.
//
// This only works with unary functions by design. If you need to have more inputs, define
// a struct type to use as a container.
func (a *Actor[Input, Output]) Call(ctx context.Context, input Input) (Output, error) {
	replyCh := make(chan reply[Output])

	a.inbox <- &message[Input, Output]{
		arg:   input,
		reply: replyCh,
	}

	select {
	case reply, ok := <-replyCh:
		if !ok {
			return z[Output](), ErrActorDied
		}

		return reply.output, reply.err
	case <-ctx.Done():
		return z[Output](), context.Cause(ctx)
	}
}
