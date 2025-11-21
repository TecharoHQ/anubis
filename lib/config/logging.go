package config

import (
	"errors"
	"fmt"
	"log/slog"
)

var (
	ErrMissingLoggingFileConfig = errors.New("config.Logging: missing value parameters in logging block")
	ErrInvalidLoggingSink       = errors.New("config.Logging: invalid sink")
	ErrInvalidLoggingFileConfig = errors.New("config.LoggingFileConfig: invalid parameters")
	ErrOutOfRange               = errors.New("config: error out of range")
)

type Logging struct {
	Sink       string             `json:"sink"`       // Logging sink, either "stdio" or "file"
	Level      *slog.Level        `json:"level"`      // Log level, if set supercedes the level in flags
	Parameters *LoggingFileConfig `json:"parameters"` // Logging parameters, to be dynamic in the future
}

const (
	LogSinkStdio = "stdio"
	LogSinkFile  = "file"
)

func (l *Logging) Valid() error {
	var errs []error

	switch l.Sink {
	case LogSinkStdio:
		// no validation needed
	case LogSinkFile:
		if l.Parameters == nil {
			errs = append(errs, ErrMissingLoggingFileConfig)
		}

		if err := l.Parameters.Valid(); err != nil {
			errs = append(errs, err)
		}
	default:
		errs = append(errs, fmt.Errorf("%w: sink %s is unknown to me", ErrInvalidLoggingSink, l.Sink))
	}

	if len(errs) != 0 {
		return errors.Join(errs...)
	}

	return nil
}

func (Logging) Default() *Logging {
	return &Logging{
		Sink: "stdio",
	}
}

type LoggingFileConfig struct {
	Filename     string `json:"file"`
	MaxBackups   int    `json:"maxBackups"`
	MaxBytes     int64  `json:"maxBytes"`
	MaxAge       int    `json:"maxAge"`
	Compress     bool   `json:"compress"`
	UseLocalTime bool   `json:"useLocalTime"`
}

func (lfc *LoggingFileConfig) Valid() error {
	if lfc == nil {
		return fmt.Errorf("logging file config is nil, why are you calling this?")
	}

	var errs []error

	if lfc.Zero() {
		errs = append(errs, ErrMissingValue)
	}

	if lfc.Filename == "" {
		errs = append(errs, fmt.Errorf("%w: filename", ErrMissingValue))
	}

	if lfc.MaxBackups < 0 {
		errs = append(errs, fmt.Errorf("%w: max backup count %d is not greater than or equal to zero", ErrOutOfRange, lfc.MaxBackups))
	}

	if lfc.MaxAge < 0 {
		errs = append(errs, fmt.Errorf("%w: max backup count %d is not greater than or equal to zero", ErrOutOfRange, lfc.MaxAge))
	}

	if len(errs) != 0 {
		errs = append([]error{ErrInvalidLoggingFileConfig}, errs...)
		return errors.Join(errs...)
	}

	return nil
}

func (lfc LoggingFileConfig) Zero() bool {
	for _, cond := range []bool{
		lfc.Filename != "",
		lfc.MaxBackups != 0,
		lfc.MaxBytes != 0,
		lfc.MaxAge != 0,
		lfc.Compress,
		lfc.UseLocalTime,
	} {
		if cond {
			return false
		}
	}

	return true
}

func (LoggingFileConfig) Default() *LoggingFileConfig {
	return &LoggingFileConfig{
		Filename:     "./var/anubis.log",
		MaxBackups:   3,
		MaxBytes:     104857600, // 100 Mi
		MaxAge:       7,         // 7 days
		Compress:     true,
		UseLocalTime: false,
	}
}
