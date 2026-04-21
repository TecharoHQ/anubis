package config

import (
	"errors"
	"fmt"
	"strconv"
)

var (
	ErrInvalidMetricsConfig     = errors.New("config: invalid metrics configuration")
	ErrNoMetricsBind            = errors.New("config.Metrics: must define bind")
	ErrNoMetricsNetwork         = errors.New("config.Metrics: must define network")
	ErrNoMetricsSocketMode      = errors.New("config.Metrics: must define socket mode when using unix sockets")
	ErrInvalidMetricsSocketMode = errors.New("config.Metrics: invalid unix socket mode")
	ErrInvalidMetricsNetwork    = errors.New("config.Metrics: invalid metrics network")
)

type Metrics struct {
	Bind       string `json:"bind" yaml:"bind"`
	Network    string `json:"network" yaml:"network"`
	SocketMode string `json:"socketMode" yaml:"socketMode"`
}

func (m *Metrics) Valid() error {
	var errs []error

	if m.Bind == "" {
		errs = append(errs, ErrNoMetricsBind)
	}

	if m.Network == "" {
		errs = append(errs, ErrNoMetricsNetwork)
	}

	switch m.Network {
	case "tcp", "tcp4", "tcp6": // https://pkg.go.dev/net#Listen
	case "unix":
		if m.SocketMode == "" {
			errs = append(errs, ErrNoMetricsSocketMode)
		}

		if _, err := strconv.ParseUint(m.SocketMode, 8, 0); err != nil {
			errs = append(errs, fmt.Errorf("%w: %w", ErrInvalidMetricsSocketMode, err))
		}
	default:
		errs = append(errs, ErrInvalidMetricsNetwork)
	}

	if len(errs) != 0 {
		return errors.Join(ErrInvalidMetricsConfig, errors.Join(errs...))
	}

	return nil
}
