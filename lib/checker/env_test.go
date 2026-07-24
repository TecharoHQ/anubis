package checker

import (
	"context"
	"testing"
)

func TestSubrequestMode(t *testing.T) {
	for _, tt := range []struct {
		name string
		ctx  func() context.Context
		want bool
	}{
		{
			name: "unset defaults to false",
			ctx:  context.Background,
			want: false,
		},
		{
			name: "explicitly enabled",
			ctx:  func() context.Context { return WithSubrequestMode(context.Background(), true) },
			want: true,
		},
		{
			name: "explicitly disabled",
			ctx:  func() context.Context { return WithSubrequestMode(context.Background(), false) },
			want: false,
		},
		{
			name: "last write wins",
			ctx: func() context.Context {
				return WithSubrequestMode(WithSubrequestMode(context.Background(), false), true)
			},
			want: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := SubrequestMode(tt.ctx()); got != tt.want {
				t.Errorf("SubrequestMode: got %v, want %v", got, tt.want)
			}
		})
	}
}
