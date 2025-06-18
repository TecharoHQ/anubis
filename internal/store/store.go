package store

import "context"

type Impl interface {
	GetInt(ctx context.Context, segments []string) (int, error)
	MultiGetInt(ctx context.Context, segments [][]string) ([]int, error)

	Increment(ctx context.Context, segments []string) error
}
