package checker

import (
	"net/http"
	"testing"
)

func TestAll_Check(t *testing.T) {
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
			name: "One not match",
			checkers: []MockChecker{
				{Result: true, Err: nil},
				{Result: false, Err: nil},
			},
			want:    false,
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
				{Result: true, Err: nil},
				{Result: false, Err: http.ErrNotSupported},
			},
			want:    false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var all All
			for _, mc := range tt.checkers {
				all = append(all, mc)
			}

			got, err := all.Check(nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("All.Check() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("All.Check() = %v, want %v", got, tt.want)
			}
		})
	}
}
