package policy

import (
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"strings"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/policy/checker"
	"github.com/gaissmai/bart"
)

var (
	ErrMisconfiguration = errors.New("[unexpected] policy: administrator misconfiguration")
)

type Checker interface {
	Check(*checker.RequestMetadata) (bool, error)
	Hash() string
}

type CheckerList []Checker

func (cl CheckerList) Check(r *checker.RequestMetadata) (bool, error) {
	for _, c := range cl {
		ok, err := c.Check(r)
		if err != nil {
			return ok, err
		}
		if ok {
			return ok, nil
		}
	}

	return false, nil
}

func (cl CheckerList) Hash() string {
	var sb strings.Builder

	for _, c := range cl {
		fmt.Fprintln(&sb, c.Hash())
	}

	return internal.SHA256sum(sb.String())
}

type RemoteAddrChecker struct {
	prefixTable *bart.Lite
	hash        string
}

func NewRemoteAddrChecker(cidrs []string) (checker.Impl, error) {
	table := new(bart.Lite)

	for _, cidr := range cidrs {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			return nil, fmt.Errorf("%w: range %s not parsing: %w", ErrMisconfiguration, cidr, err)
		}

		table.Insert(prefix)
	}

	return &RemoteAddrChecker{
		prefixTable: table,
		hash:        internal.FastHash(strings.Join(cidrs, ",")),
	}, nil
}

func (rac *RemoteAddrChecker) Check(r *checker.RequestMetadata) (bool, error) {
	addr, ok := netip.AddrFromSlice(r.RemoteAddr)
	if !ok {
		return false, fmt.Errorf("%w: %s is not an IP address", ErrMisconfiguration, r.RemoteAddr)
	}

	// Convert IPv4-mapped IPv6 addresses to IPv4
	if addr.Is6() && addr.Is4In6() {
		addr = addr.Unmap()
	}

	return rac.prefixTable.Contains(addr), nil
}

func (rac *RemoteAddrChecker) Hash() string {
	return rac.hash
}

type HeaderMatchesChecker struct {
	header string
	regexp *regexp.Regexp
	hash   string
}

func NewUserAgentChecker(rexStr string) (checker.Impl, error) {
	return NewHeaderMatchesChecker("User-Agent", rexStr)
}

func NewHeaderMatchesChecker(header, rexStr string) (checker.Impl, error) {
	rex, err := regexp.Compile(strings.TrimSpace(rexStr))
	if err != nil {
		return nil, fmt.Errorf("%w: regex %s failed parse: %w", ErrMisconfiguration, rexStr, err)
	}
	return &HeaderMatchesChecker{strings.TrimSpace(header), rex, internal.FastHash(header + ": " + rexStr)}, nil
}

func (hmc *HeaderMatchesChecker) Check(r *checker.RequestMetadata) (bool, error) {
	if hmc.regexp.MatchString(r.Header.Get(hmc.header)) {
		return true, nil
	}

	return false, nil
}

func (hmc *HeaderMatchesChecker) Hash() string {
	return hmc.hash
}

type PathChecker struct {
	regexp *regexp.Regexp
	hash   string
}

func NewPathChecker(rexStr string) (checker.Impl, error) {
	rex, err := regexp.Compile(strings.TrimSpace(rexStr))
	if err != nil {
		return nil, fmt.Errorf("%w: regex %s failed parse: %w", ErrMisconfiguration, rexStr, err)
	}
	return &PathChecker{rex, internal.FastHash(rexStr)}, nil
}

func (pc *PathChecker) Check(r *checker.RequestMetadata) (bool, error) {
	originalUrl := r.Header.Get("X-Original-URI")
	if originalUrl != "" {
		if pc.regexp.MatchString(originalUrl) {
			return true, nil
		}
	}

	if pc.regexp.MatchString(r.Path) {
		return true, nil
	}

	return false, nil
}

func (pc *PathChecker) Hash() string {
	return pc.hash
}

func NewHeaderExistsChecker(key string) checker.Impl {
	return headerExistsChecker{strings.TrimSpace(key)}
}

type headerExistsChecker struct {
	header string
}

func (hec headerExistsChecker) Check(r *checker.RequestMetadata) (bool, error) {
	if r.Header.Get(hec.header) != "" {
		return true, nil
	}

	return false, nil
}

func (hec headerExistsChecker) Hash() string {
	return internal.FastHash(hec.header)
}

func NewHeadersChecker(headermap map[string]string) (checker.Impl, error) {
	var result checker.List
	var errs []error

	for key, rexStr := range headermap {
		if rexStr == ".*" {
			result = append(result, headerExistsChecker{strings.TrimSpace(key)})
			continue
		}

		rex, err := regexp.Compile(strings.TrimSpace(rexStr))
		if err != nil {
			errs = append(errs, fmt.Errorf("while compiling header %s regex %s: %w", key, rexStr, err))
			continue
		}

		result = append(result, &HeaderMatchesChecker{key, rex, internal.FastHash(key + ": " + rexStr)})
	}

	if len(errs) != 0 {
		return nil, errors.Join(errs...)
	}

	return result, nil
}
