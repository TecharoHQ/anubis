package remoteaddress

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/netip"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/checker"
	"github.com/gaissmai/bart"
)

var (
	ErrNoRemoteAddresses = errors.New("remoteaddress: no remote addresses defined")
	ErrInvalidCIDR       = errors.New("remoteaddress: invalid CIDR")
)

func init() {
	checker.Register("remote_address", Factory{})
}

type Factory struct{}

func (Factory) Valid(_ context.Context, inp json.RawMessage) error {
	var fc fileConfig
	if err := json.Unmarshal([]byte(inp), &fc); err != nil {
		return fmt.Errorf("%w: %w", checker.ErrUnparseableConfig, err)
	}

	if err := fc.Valid(); err != nil {
		return err
	}

	return nil
}

func (Factory) Build(_ context.Context, inp json.RawMessage) (checker.Interface, error) {
	c := struct {
		RemoteAddr []netip.Prefix `json:"remote_addresses,omitempty" yaml:"remote_addresses,omitempty"`
	}{}

	if err := json.Unmarshal([]byte(inp), &c); err != nil {
		return nil, fmt.Errorf("%w: %w", checker.ErrUnparseableConfig, err)
	}

	table := new(bart.Lite)

	for _, cidr := range c.RemoteAddr {
		table.Insert(cidr)
	}

	return &RemoteAddrChecker{
		prefixTable: table,
		hash:        internal.FastHash(string(inp)),
	}, nil
}

type fileConfig struct {
	RemoteAddr []string `json:"remote_addresses,omitempty" yaml:"remote_addresses,omitempty"`
}

func (fc fileConfig) Valid() error {
	var errs []error

	if len(fc.RemoteAddr) == 0 {
		errs = append(errs, ErrNoRemoteAddresses)
	}

	for _, cidr := range fc.RemoteAddr {
		if _, err := netip.ParsePrefix(cidr); err != nil {
			errs = append(errs, fmt.Errorf("%w: cidr %q is invalid: %w", ErrInvalidCIDR, cidr, err))
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("%w: %w", checker.ErrInvalidConfig, errors.Join(errs...))
	}

	return nil
}

func Valid(cidrs []string) error {
	fc := fileConfig{
		RemoteAddr: cidrs,
	}

	return fc.Valid()
}

func New(cidrs []string) (checker.Interface, error) {
	fc := fileConfig{
		RemoteAddr: cidrs,
	}
	data, err := json.Marshal(fc)
	if err != nil {
		return nil, err
	}

	return Factory{}.Build(context.Background(), json.RawMessage(data))
}

type RemoteAddrChecker struct {
	prefixTable *bart.Lite
	hash        string
}

func (rac *RemoteAddrChecker) Check(r *http.Request) (bool, error) {
	host := r.Header.Get("X-Real-Ip")
	if host == "" {
		return false, fmt.Errorf("%w: header X-Real-Ip is not set", anubis.ErrMisconfiguration)
	}

	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false, fmt.Errorf("%w: %s is not an IP address: %w", anubis.ErrMisconfiguration, host, err)
	}

	return rac.prefixTable.Contains(addr), nil
}

func (rac *RemoteAddrChecker) Hash() string {
	return rac.hash
}
