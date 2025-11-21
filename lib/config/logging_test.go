package config

import (
	"errors"
	"testing"
)

func TestLoggingValid(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input *Logging
		want  error
	}{
		{
			name:  "simple happy",
			input: (Logging{}).Default(),
		},
		{
			name: "default file config",
			input: &Logging{
				Sink:       LogSinkFile,
				Parameters: (&LoggingFileConfig{}).Default(),
			},
		},
		{
			name: "invalid sink",
			input: &Logging{
				Sink: "taco invalid",
			},
			want: ErrInvalidLoggingSink,
		},
		{
			name: "missing parameters",
			input: &Logging{
				Sink: LogSinkFile,
			},
			want: ErrMissingLoggingFileConfig,
		},
		{
			name: "invalid parameters",
			input: &Logging{
				Sink:       LogSinkFile,
				Parameters: &LoggingFileConfig{},
			},
			want: ErrInvalidLoggingFileConfig,
		},
		{
			name: "file sink with no filename",
			input: &Logging{
				Sink: LogSinkFile,
				Parameters: &LoggingFileConfig{
					Filename:     "",
					MaxBackups:   3,
					MaxBytes:     104857600, // 100 Mi
					MaxAge:       7,         // 7 days
					Compress:     true,
					UseLocalTime: false,
				},
			},
			want: ErrMissingValue,
		},
		{
			name: "file sink with negative max backups",
			input: &Logging{
				Sink: LogSinkFile,
				Parameters: &LoggingFileConfig{
					Filename:     "./var/anubis.log",
					MaxBackups:   -3,
					MaxBytes:     104857600, // 100 Mi
					MaxAge:       7,         // 7 days
					Compress:     true,
					UseLocalTime: false,
				},
			},
			want: ErrOutOfRange,
		},
		{
			name: "file sink with negative max age",
			input: &Logging{
				Sink: LogSinkFile,
				Parameters: &LoggingFileConfig{
					Filename:     "./var/anubis.log",
					MaxBackups:   3,
					MaxBytes:     104857600, // 100 Mi
					MaxAge:       -7,        // 7 days
					Compress:     true,
					UseLocalTime: false,
				},
			},
			want: ErrOutOfRange,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Valid()

			if !errors.Is(err, tt.want) {
				t.Logf("wanted error: %v", tt.want)
				t.Logf("   got error: %v", err)
				t.Fatal("got wrong error")
			}
		})
	}
}
