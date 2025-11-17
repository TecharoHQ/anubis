package checker

import (
	"errors"
	"net/http"
	"testing"
)

// Mock implements the Impl interface for testing.
type Mock struct {
	result bool
	err    error
	hash   string
}

func (m Mock) Check(r *http.Request) (bool, error) { return m.result, m.err }
func (m Mock) Hash() string                        { return m.hash }

func TestListCheck_AndSemantics(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)

	tests := []struct {
		name    string
		list    List
		want    bool
		wantErr bool
	}{
		{
			name: "all true",
			list: List{Mock{true, nil, "a"}, Mock{true, nil, "b"}},
			want: true,
		},
		{
			name: "one false",
			list: List{Mock{true, nil, "a"}, Mock{false, nil, "b"}},
			want: false,
		},
		{
			name:    "error propagates",
			list:    List{Mock{true, nil, "a"}, Mock{true, errors.New("boom"), "b"}},
			want:    false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.list.Check(req)
			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error state: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
