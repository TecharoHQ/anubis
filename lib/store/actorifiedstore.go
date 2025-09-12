package store

import (
	"context"
	"time"

	"github.com/TecharoHQ/anubis/internal/actorify"
)

type unit struct{}

type ActorifiedStore struct {
	Interface

	deleteActor *actorify.Actor[string, unit]
	getActor    *actorify.Actor[string, []byte]
	setActor    *actorify.Actor[*actorSetReq, unit]
	cancel      context.CancelFunc
}

type actorSetReq struct {
	key    string
	value  []byte
	expiry time.Duration
}

func NewActorifiedStore(backend Interface) *ActorifiedStore {
	ctx, cancel := context.WithCancel(context.Background())

	result := &ActorifiedStore{
		Interface: backend,
		cancel:    cancel,
	}

	result.deleteActor = actorify.New(ctx, result.actorDelete)
	result.getActor = actorify.New(ctx, backend.Get)
	result.setActor = actorify.New(ctx, result.actorSet)

	return result
}

func (a *ActorifiedStore) Close() { a.cancel() }

func (a *ActorifiedStore) Delete(ctx context.Context, key string) error {
	if _, err := a.deleteActor.Call(ctx, key); err != nil {
		return err
	}

	return nil
}

func (a *ActorifiedStore) Get(ctx context.Context, key string) ([]byte, error) {
	return a.getActor.Call(ctx, key)
}

func (a *ActorifiedStore) Set(ctx context.Context, key string, value []byte, expiry time.Duration) error {
	if _, err := a.setActor.Call(ctx, &actorSetReq{
		key:    key,
		value:  value,
		expiry: expiry,
	}); err != nil {
		return err
	}

	return nil
}

func (a *ActorifiedStore) actorDelete(ctx context.Context, key string) (unit, error) {
	if err := a.Interface.Delete(ctx, key); err != nil {
		return unit{}, err
	}

	return unit{}, nil
}

func (a *ActorifiedStore) actorSet(ctx context.Context, req *actorSetReq) (unit, error) {
	if err := a.Interface.Set(ctx, req.key, req.value, req.expiry); err != nil {
		return unit{}, err
	}

	return unit{}, nil
}
