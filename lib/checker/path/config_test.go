package path

import (
	"errors"
	"testing"
)

func TestFileConfigValid(t *testing.T) {
	for _, tt := range []struct {
		name, description string
		in                fileConfig
		err               error
	}{
		{
			name:        "simple happy",
			description: "the most common usecase",
			in: fileConfig{
				Regex: "^/api/.*",
			},
		},
		{
			name:        "wildcard match",
			description: "match files with specific extension",
			in: fileConfig{
				Regex: ".*[.]json$",
			},
		},
		{
			name:        "no regex",
			description: "Regex must be set, it is not",
			in:          fileConfig{},
			err:         ErrNoRegex,
		},
		{
			name:        "invalid regex",
			description: "the user wrote an invalid regular expression",
			in: fileConfig{
				Regex: "[a-z",
			},
			err: ErrInvalidRegex,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.in.Valid(); !errors.Is(err, tt.err) {
				t.Log(tt.description)
				t.Fatalf("got %v, wanted %v", err, tt.err)
			}
		})
	}
}
