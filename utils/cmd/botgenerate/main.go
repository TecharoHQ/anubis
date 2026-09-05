package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/facebookgo/flagenv"
	"gopkg.in/yaml.v3"
)

var (
	dataDir   = flag.String("data-dir", "data/crawlers", "folder holding the crawler policy files")
	dryRun    = flag.Bool("dry-run", false, "report what would change but don't write any files")
	list      = flag.Bool("list", false, "list every crawler this tool can update, then exit")
	slogLevel = flag.String("slog-level", "INFO", "logging level (DEBUG, INFO, WARN, ERROR)")
	timeout   = flag.Duration("timeout", 30*time.Second, "how long to wait for a single feed")
	userAgent = flag.String("user-agent", "Anubis-botgenerate (+https://github.com/TecharoHQ/anubis)", "User-Agent to send when fetching feeds")
)

var (
	ErrNoSuchCrawler     = errors.New("botgenerate: no crawler by that name")
	ErrRuleNotFound      = errors.New("botgenerate: policy file has no rule by that name")
	ErrNoRemoteAddresses = errors.New("botgenerate: rule has no remote_addresses field")
	ErrNoPrefixes        = errors.New("botgenerate: feed returned no usable prefixes")
	ErrUnexpectedStatus  = errors.New("botgenerate: feed returned an unexpected HTTP status")
)

// Source says where one crawler's IP range list comes from.
type Source struct {
	// File is the policy file to update, relative to --data-dir. Its name
	// without the extension is how the crawler is named on the command line.
	File string

	// Rule is the name of the rule inside File whose remote_addresses list gets
	// replaced. A policy file may hold more than one rule.
	Rule string

	// URL is the operator's published feed.
	URL string
}

// Name is how this crawler is spelled on the command line.
func (s Source) Name() string {
	return strings.TrimSuffix(s.File, filepath.Ext(s.File))
}

func (s Source) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("file", s.File),
		slog.String("rule", s.Rule),
		slog.String("url", s.URL),
	)
}

var sources = []Source{
	{
		File: "applebot.yaml",
		Rule: "applebot",
		URL:  "https://search.developer.apple.com/applebot.json",
	},
	{
		File: "bingbot.yaml",
		Rule: "bingbot",
		URL:  "https://www.bing.com/toolbox/bingbot.json",
	},
	{
		File: "commoncrawl.yaml",
		Rule: "common-crawl",
		URL:  "https://index.commoncrawl.org/ccbot.json",
	},
	{
		File: "duckduckbot.yaml",
		Rule: "duckduckbot",
		URL:  "https://duckduckgo.com/duckduckbot.json",
	},
	{
		// The list of Google crawlers that respect robots.txt
		// https://developers.google.com/crawling/docs/crawlers-fetchers/verify-google-requests
		File: "googlebot.yaml",
		Rule: "googlebot",
		URL:  "https://developers.google.com/static/crawling/ipranges/common-crawlers.json",
	},
	{
		File: "mojeekbot.yaml",
		Rule: "mojeekbot",
		URL:  "https://www.mojeek.com/mojeekbot.json",
	},
	{
		File: "openai-gptbot.yaml",
		Rule: "openai-gptbot",
		URL:  "https://openai.com/gptbot.json",
	},
	{
		File: "openai-searchbot.yaml",
		Rule: "openai-searchbot",
		URL:  "https://openai.com/searchbot.json",
	},
	{
		File: "perplexitybot.yaml",
		Rule: "perplexitybot",
		URL:  "https://www.perplexity.com/perplexitybot.json",
	},
	{
		File: "qwantbot.yaml",
		Rule: "qwantbot",
		URL:  "https://help.qwant.com/wp-content/uploads/sites/latest/qwantbot.json",
	},
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), `Usage of %[1]s:

	%[1]s [flags] [crawler...]

Updates the remote_addresses list of each crawler from the IP range lists
their operators publish. If no crawler is passed, it updates everything.

After running this, run npm run format

Flags:
`, filepath.Base(os.Args[0]))

		flag.PrintDefaults()
	}
}

func main() {
	flagenv.Parse()
	flag.Parse()

	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(*slogLevel)); err != nil {
		fmt.Fprintf(os.Stderr, "--slog-level=%q is invalid: %v\n", *slogLevel, err)
		os.Exit(2)
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})))

	if *list {
		for _, src := range sources {
			fmt.Printf("%-20s %s\n", src.Name(), src.URL)
		}
		return
	}

	want, err := selectSources(flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n\nRun %s --list to see every known crawler.\n", err, filepath.Base(os.Args[0]))
		os.Exit(2)
	}

	u := &updater{
		cli:       &http.Client{Timeout: *timeout},
		dataDir:   *dataDir,
		dryRun:    *dryRun,
		userAgent: *userAgent,
	}

	var errs []error
	for _, src := range want {
		if err := u.update(src); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", src.Name(), err))
		}
	}

	if len(errs) != 0 {
		slog.Error("could not update every crawler", "err", errors.Join(errs...))
		os.Exit(1)
	}
}

func selectSources(names []string) ([]Source, error) {
	if len(names) == 0 {
		return sources, nil
	}

	var (
		want []Source
		errs []error
	)

	for _, name := range names {
		idx := slices.IndexFunc(sources, func(s Source) bool { return s.Name() == name })
		if idx == -1 {
			errs = append(errs, fmt.Errorf("%w: %q", ErrNoSuchCrawler, name))
			continue
		}

		want = append(want, sources[idx])
	}

	if len(errs) != 0 {
		return nil, errors.Join(errs...)
	}

	return want, nil
}

type updater struct {
	cli       *http.Client
	dataDir   string
	dryRun    bool
	userAgent string
}

func (u *updater) update(src Source) error {
	lg := slog.With("crawler", src.Name())

	prefixes, err := u.fetchPrefixes(src.URL)
	if err != nil {
		return err
	}

	fname := filepath.Join(u.dataDir, src.File)

	data, err := os.ReadFile(fname)
	if err != nil {
		return fmt.Errorf("can't read policy file: %w", err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("can't parse %s: %w", fname, err)
	}

	old, err := setRemoteAddresses(&root, src.Rule, prefixes)
	if err != nil {
		return fmt.Errorf("can't update %s: %w", fname, err)
	}

	added, removed := diff(old, prefixes)
	if len(added) == 0 && len(removed) == 0 {
		lg.Info("already up to date", "prefixes", len(prefixes))
		return nil
	}

	lg.Info("IP ranges changed", "added", len(added), "removed", len(removed), "total", len(prefixes))
	lg.Debug("IP ranges changed", "added", added, "removed", removed)

	if u.dryRun {
		return nil
	}

	out, err := marshal(&root)
	if err != nil {
		return fmt.Errorf("can't serialize %s: %w", fname, err)
	}

	if err := os.WriteFile(fname, out, 0o644); err != nil {
		return fmt.Errorf("can't write %s: %w", fname, err)
	}

	lg.Info("wrote policy file", "file", fname)

	return nil
}

// prefixList is the common subset of JSON fields in search engine IP prefix feeds
type prefixList struct {
	Prefixes []struct {
		IPv4Prefix string `json:"ipv4Prefix"`
		IPv6Prefix string `json:"ipv6Prefix"`
	} `json:"prefixes"`
}

func (u *updater) fetchPrefixes(url string) ([]string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("can't build request for %s: %w", url, err)
	}
	req.Header.Set("User-Agent", u.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := u.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("can't fetch %s: %w", url, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %s returned %s", ErrUnexpectedStatus, url, resp.Status)
	}

	prefixes, err := parsePrefixes(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("can't read %s: %w", url, err)
	}

	return prefixes, nil
}

func parsePrefixes(r io.Reader) ([]string, error) {
	var pl prefixList
	if err := json.NewDecoder(r).Decode(&pl); err != nil {
		return nil, fmt.Errorf("can't decode prefix list: %w", err)
	}

	var prefixes []string
	for i, p := range pl.Prefixes {
		var raw string
		switch {
		case p.IPv4Prefix != "":
			raw = p.IPv4Prefix
		case p.IPv6Prefix != "":
			raw = p.IPv6Prefix
		default:
			return nil, fmt.Errorf("prefix %d has neither ipv4Prefix nor ipv6Prefix", i)
		}

		pfx, err := netip.ParsePrefix(raw)
		if err != nil {
			return nil, fmt.Errorf("prefix %d (%q) is not a CIDR range: %w", i, raw, err)
		}

		prefixes = append(prefixes, pfx.String())
	}

	if len(prefixes) == 0 {
		return nil, ErrNoPrefixes
	}

	return prefixes, nil
}

func setRemoteAddresses(root *yaml.Node, rule string, prefixes []string) ([]string, error) {
	if len(root.Content) == 0 {
		return nil, fmt.Errorf("%w: %s (file is empty)", ErrRuleNotFound, rule)
	}

	for _, node := range root.Content[0].Content {
		name := mappingValue(node, "name")
		if name == nil || name.Value != rule {
			continue
		}

		addrs := mappingValue(node, "remote_addresses")
		if addrs == nil {
			return nil, fmt.Errorf("%w: %s", ErrNoRemoteAddresses, rule)
		}

		old := make([]string, 0, len(addrs.Content))
		for _, item := range addrs.Content {
			old = append(old, item.Value)
		}

		var style yaml.Style
		if addrs.Style&yaml.FlowStyle != 0 {
			style = yaml.DoubleQuotedStyle
		}

		content := make([]*yaml.Node, 0, len(prefixes))
		for _, prefix := range prefixes {
			content = append(content, &yaml.Node{
				Kind:  yaml.ScalarNode,
				Tag:   "!!str",
				Value: prefix,
				Style: style,
			})
		}

		addrs.Kind = yaml.SequenceNode
		addrs.Tag = "!!seq"
		addrs.Content = content

		return old, nil
	}

	return nil, fmt.Errorf("%w: %s", ErrRuleNotFound, rule)
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}

	return nil
}

func marshal(root *yaml.Node) ([]byte, error) {
	var buf strings.Builder

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	if err := enc.Encode(root); err != nil {
		return nil, err
	}

	if err := enc.Close(); err != nil {
		return nil, err
	}

	return []byte(buf.String()), nil
}

func diff(before, after []string) (added, removed []string) {
	inBefore := make(map[string]struct{}, len(before))
	for _, prefix := range before {
		inBefore[prefix] = struct{}{}
	}

	inAfter := make(map[string]struct{}, len(after))
	for _, prefix := range after {
		inAfter[prefix] = struct{}{}
	}

	for _, prefix := range after {
		if _, ok := inBefore[prefix]; !ok {
			added = append(added, prefix)
		}
	}

	for _, prefix := range before {
		if _, ok := inAfter[prefix]; !ok {
			removed = append(removed, prefix)
		}
	}

	return added, removed
}
