package headermatches

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
				Header:     "User-Agent",
				ValueRegex: ".*",
			},
		},
		{
			name:        "no header",
			description: "Header must be set, it is not",
			in: fileConfig{
				ValueRegex: ".*",
			},
			err: ErrNoHeader,
		},
		{
			name:        "no value regex",
			description: "ValueRegex must be set, it is not",
			in: fileConfig{
				Header: "User-Agent",
			},
			err: ErrNoValueRegex,
		},
		{
			name:        "invalid regex",
			description: "the user wrote an invalid value regular expression",
			in: fileConfig{
				Header:     "User-Agent",
				ValueRegex: "[a-z",
			},
			err: ErrInvalidRegex,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.in.Valid(); !errors.Is(err, tt.err) {
				t.Log(tt.description)
				t.Fatal(err)
			}
		})
	}
}
