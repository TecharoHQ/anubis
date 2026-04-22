package config

import (
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"strconv"
)

var (
	ErrInvalidMetricsConfig     = errors.New("config: invalid metrics configuration")
	ErrInvalidMetricsTLSConfig  = errors.New("config: invalid metrics TLS configuration")
	ErrNoMetricsBind            = errors.New("config.Metrics: must define bind")
	ErrNoMetricsNetwork         = errors.New("config.Metrics: must define network")
	ErrNoMetricsSocketMode      = errors.New("config.Metrics: must define socket mode when using unix sockets")
	ErrInvalidMetricsSocketMode = errors.New("config.Metrics: invalid unix socket mode")
	ErrInvalidMetricsNetwork    = errors.New("config.Metrics: invalid metrics network")
	ErrNoMetricsTLSCertificate  = errors.New("config.Metrics.TLS: must define certificate file")
	ErrNoMetricsTLSKey          = errors.New("config.Metrics.TLS: must define key file")
	ErrInvalidMetricsTLSKeypair = errors.New("config.Metrics.TLS: keypair is invalid")
	ErrCantReadFile             = errors.New("config: can't read required file")
)

type Metrics struct {
	Bind       string      `json:"bind" yaml:"bind"`
	Network    string      `json:"network" yaml:"network"`
	SocketMode string      `json:"socketMode" yaml:"socketMode"`
	TLS        *MetricsTLS `json:"tls" yaml:"tls"`
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

	if m.TLS != nil {
		if err := m.TLS.Valid(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return errors.Join(ErrInvalidMetricsConfig, errors.Join(errs...))
	}

	return nil
}

type MetricsTLS struct {
	Certificate string `json:"certificate" yaml:"certificate"`
	Key         string `json:"key" yaml:"key"`
}

func (mt *MetricsTLS) Valid() error {
	var errs []error

	if mt.Certificate == "" {
		errs = append(errs, ErrNoMetricsTLSCertificate)
	}

	if err := canReadFile(mt.Certificate); err != nil {
		errs = append(errs, fmt.Errorf("%w %s: %w", ErrCantReadFile, mt.Certificate, err))
	}

	if mt.Key == "" {
		errs = append(errs, ErrNoMetricsTLSKey)
	}

	if err := canReadFile(mt.Key); err != nil {
		errs = append(errs, fmt.Errorf("%w %s: %w", ErrCantReadFile, mt.Key, err))
	}

	if _, err := tls.LoadX509KeyPair(mt.Certificate, mt.Key); err != nil {
		errs = append(errs, fmt.Errorf("%w: %w", ErrInvalidMetricsTLSKeypair, err))
	}

	if len(errs) != 0 {
		return errors.Join(ErrInvalidMetricsTLSConfig, errors.Join(errs...))
	}

	return nil
}

func canReadFile(fname string) error {
	fin, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer fin.Close()

	data := make([]byte, 64)
	if _, err := fin.Read(data); err != nil {
		return fmt.Errorf("can't read %s: %w", fname, err)
	}

	return nil
}
