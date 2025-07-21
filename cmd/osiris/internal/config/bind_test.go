package config

import (
	"errors"
	"net"
	"testing"
)

func TestBindValid(t *testing.T) {
	for _, tt := range []struct {
		name         string
		precondition func(t *testing.T)
		bind         Bind
		err          error
	}{
		{
			name:         "basic",
			precondition: nil,
			bind: Bind{
				HTTP:    ":8081",
				HTTPS:   ":8082",
				Metrics: ":8083",
			},
			err: nil,
		},
		{
			name: "invalid ports",
			precondition: func(t *testing.T) {
				ln, err := net.Listen("tcp", ":8081")
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { ln.Close() })
			},
			bind: Bind{
				HTTP:    "",
				HTTPS:   "",
				Metrics: "",
			},
			err: ErrInvalidHostpost,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.precondition != nil {
				tt.precondition(t)
			}

			if err := tt.bind.Valid(); !errors.Is(err, tt.err) {
				t.Logf("want: %v", tt.err)
				t.Logf("got:  %v", err)
				t.Error("got wrong error from validation function")
			}
		})
	}
}
