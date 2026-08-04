package bundler

import (
	"context"
	"testing"
)

func TestBundler(t *testing.T) {
	input := []int{1, 2, 3, 4}
	done := false
	b := New[int](func(_ context.Context, data []int) {
		if len(data) != 4 {
			t.Errorf("Wanted len(data) == %d, got: %d", len(input), len(data))
		}

		sum := 0
		const wantSum = 10
		for _, i := range data {
			sum += i
		}

		if sum != wantSum {
			t.Errorf("wanted sum of inputs to be %d, got: %d", wantSum, sum)
		}
		done = true
	})

	for _, i := range input {
		b.Add(i, 1)
	}

	b.Flush()

	if !done {
		t.Fatal("function wasn't called")
	}
}
