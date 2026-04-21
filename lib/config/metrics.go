package config

import "errors"

var (
	ErrInvalidMetricsConfig  = errors.New("config: invalid metrics configuration")
	ErrNoMetricsBind         = errors.New("config.Metrics: must define bind")
	ErrNoMetricsNetwork      = errors.New("config.Metrics: must define network")
	ErrInvalidMetricsNetwork = errors.New("config.Metrics: invalid metrics network")
)

type Metrics struct {
	Bind    string `json:"bind" yaml:"bind"`
	Network string `json:"network" yaml:"network"`
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
	case "tcp", "tcp4", "tcp6", "unix": // https://pkg.go.dev/net#Listen
	default:
		errs = append(errs, ErrInvalidMetricsNetwork)
	}

	if len(errs) != 0 {
		return errors.Join(ErrInvalidMetricsConfig, errors.Join(errs...))
	}

	return nil
}
