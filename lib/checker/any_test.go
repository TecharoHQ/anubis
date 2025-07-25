package checker

import (
	"net/http"
	"testing"
)

type MockChecker struct {
	Result bool
	Err    error
}

func (m MockChecker) Check(r *http.Request) (bool, error) {
	return m.Result, m.Err
}

func (m MockChecker) Hash() string {
	return "mock-hash"
}

func TestAny_Check(t *testing.T) {
	tests := []struct {
		name     string
		checkers []MockChecker
		want     bool
		wantErr  bool
	}{
		{
			name: "All match",
			checkers: []MockChecker{
				{Result: true, Err: nil},
				{Result: true, Err: nil},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "One match",
			checkers: []MockChecker{
				{Result: false, Err: nil},
				{Result: true, Err: nil},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "No match",
			checkers: []MockChecker{
				{Result: false, Err: nil},
				{Result: false, Err: nil},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "Error encountered",
			checkers: []MockChecker{
				{Result: false, Err: nil},
				{Result: false, Err: http.ErrNotSupported},
			},
			want:    false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var any Any
			for _, mc := range tt.checkers {
				any = append(any, mc)
			}

			got, err := any.Check(nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("Any.Check() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Any.Check() = %v, want %v", got, tt.want)
			}
		})
	}
}
