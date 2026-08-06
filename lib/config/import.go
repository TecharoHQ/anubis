package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/TecharoHQ/anubis/data"
	"github.com/TecharoHQ/anubis/internal/multifile"
	"github.com/fvbommel/sortorder"
	"github.com/goreleaser/fileglob"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var (
	ErrInvalidImportStatement = errors.New("config.ImportStatement: invalid source file")
	ErrCantReadImportedFile   = errors.New("config.ImportStatement: can't read imported file")
	ErrGlobMatchedNothing     = errors.New("config.ImportStatement: glob pattern matched no files")
)

type ImportStatement struct {
	Import string `json:"import"`
	Bots   []BotConfig
}

const globHints = `recursive wildcards ("**") and alternation ("{yaml,yml}") are not supported`

func globMatch(globbedPath string) (fs.FS, []string, error) {
	var fsys fs.FS = nil

	if after, ok := strings.CutPrefix(globbedPath, "(data)/"); ok {
		globbedPath = after
		fsys = data.BotPolicies
	}

	var matches []string
	var err error

	switch {
	case fsys == nil:
		matches, err = filepath.Glob(globbedPath)
	case fsys != nil:
		matches, err = fs.Glob(fsys, globbedPath)
	}

	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", ErrInvalidImportStatement, err)
	}

	if len(matches) == 0 {
		if strings.Contains(globbedPath, "**") || strings.Contains(globbedPath, "{") {
			return nil, nil, fmt.Errorf("%w: note that %s", ErrGlobMatchedNothing, globHints)
		}
		return nil, nil, ErrGlobMatchedNothing
	}

	sort.Sort(sortorder.Natural(matches))

	var errs []error

	for _, fname := range matches {
		switch {
		case fsys == nil:
			if _, err := os.Stat(fname); err != nil {
				errs = append(errs, fmt.Errorf("%w: %q", ErrCantReadImportedFile, fname))
			}
		case fsys != nil:
			if _, err := fs.Stat(fsys, fname); err != nil {
				errs = append(errs, fmt.Errorf("%w: %q", ErrCantReadImportedFile, fname))
			}
		}
	}

	if len(errs) != 0 {
		return nil, nil, fmt.Errorf("%w: %w", ErrInvalidImportStatement, errors.Join(errs...))
	}

	return fsys, matches, nil
}

func (is *ImportStatement) open() (fs.File, error) {
	if fileglob.ContainsMatchers(is.Import) {
		fsys, fnames, err := globMatch(is.Import)
		if err != nil {
			return nil, err
		}

		return multifile.YAMLList(fsys, fnames)
	}

	if after, ok := strings.CutPrefix(is.Import, "(data)/"); ok {
		fname := after
		fin, err := data.BotPolicies.Open(fname)
		return fin, err
	}

	return os.Open(is.Import)
}

func (is *ImportStatement) load() error {
	fin, err := is.open()
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrInvalidImportStatement, is.Import, err)
	}
	defer fin.Close() //nolint:errcheck

	var imported []BotOrImport
	var result []BotConfig

	dec := yaml.NewYAMLToJSONDecoder(fin)
	for {
		var doc []BotOrImport
		if err := dec.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("%w: can't parse %s: %w", ErrInvalidImportStatement, is.Import, err)
		}

		imported = append(imported, doc...)
	}

	var errs []error

	for _, b := range imported {
		if err := b.Valid(); err != nil {
			errs = append(errs, err)
		}

		if b.ImportStatement != nil {
			result = append(result, b.Bots...)
		}

		if b.BotConfig != nil {
			result = append(result, *b.BotConfig)
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("config %s is not valid:\n%w", is.Import, errors.Join(errs...))
	}

	is.Bots = result

	return nil
}

func (is *ImportStatement) Valid() error {
	return is.load()
}
