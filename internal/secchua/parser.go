package secchua

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Parse parses a Sec-Ch-Ua header into brand/version pairs, preserving order and GREASE entries.
// An empty or whitespace only header parses to no version paris.
//
// Brands with no MajorVersion get the sentinel version "?1" (true).
func Parse(header string) ([]Version, error) {
	p := &parser{in: header}
	var result []Version

	p.skipOptionalWhitespace()
	if p.eof() {
		return nil, nil
	}

	for {
		name, err := p.bareItem()
		if err != nil {
			return nil, fmt.Errorf("%w (bareItem)", err)
		}

		params, err := p.params()
		if err != nil {
			return nil, fmt.Errorf("%w (params)", err)
		}

		result = append(result, Version{Name: name, Version: params["v"]})

		p.skipOptionalWhitespace()
		if p.eof() {
			return result, nil
		}
		if got := p.in[p.pos]; got != ',' {
			return nil, fmt.Errorf("%w: %w want: %q at offset %d, got: %q (after result appended)", ErrParserSyntax, ErrIllegalByte, ",", p.pos, got)
		}
		p.pos++
		p.skipOptionalWhitespace()
		if p.eof() {
			return nil, fmt.Errorf("%w: %w at offset %d", ErrParserSyntax, ErrTrailingComma, p.pos)
		}
	}
}

// Canonicalize sorts parsed [Version]s by name -> version and drops GREASE entries.
//
// This uses plain byte sorting so that the result is stable across releases and machines.
func Canonicalize(in []Version) []Version {
	result := make([]Version, 0, len(in)) // capacity for up to len(in) Version entries
	for _, v := range in {
		if IsGREASE(v.Name) {
			continue
		}
		result = append(result, v)
	}

	slices.SortFunc(result, func(lhs, rhs Version) int {
		if c := cmp.Compare(lhs.Name, rhs.Name); c != 0 {
			return c
		}

		return cmp.Compare(lhs.Version, rhs.Version)
	})

	return slices.Clip(result)
}

// ParseCanonical threads Parse and Canonicalize in one step.
func ParseCanonical(header string) ([]Version, error) {
	result, err := Parse(header)
	if err != nil {
		return nil, err
	}

	return Canonicalize(result), nil
}

var (
	// ErrParserSyntax is returned when a header cannot be parsed.
	ErrParserSyntax = errors.New("secchua: syntax error")
	// ErrEmptyItem is returned when a bare item is empty.
	ErrEmptyItem = errors.New("empty item at offset")
	// ErrIllegalEscape is returned when an illegal escape sequence is used.
	ErrIllegalEscape = errors.New("illegal escape sequence")
	// ErrIllegalByte is returned when an illegal byte is present in a field.
	ErrIllegalByte = errors.New("illegal byte")
	// ErrUnterminatedString is returned when a string isn't properly terminated.
	ErrUnterminatedString = errors.New("unterminated string")
	// ErrEmptyParameterKey is returned when a parameter key is empty.
	ErrEmptyParameterKey = errors.New("empty parameter key")
	// ErrTrailingComma is returned when a version pair has a trailing comma.
	ErrTrailingComma = errors.New("trailing comma")
)

const (
	bareFalse = "?0"
	bareTrue  = "?1"
)

// parser parses the given string into brand/version pairs.
//
// The syntax follows RFC 8941[1] and RFC 9651[2].
//
// [1]: https://www.rfc-editor.org/info/rfc8941/
// [2]: https://www.rfc-editor.org/info/rfc9651/
type parser struct {
	in  string
	pos int
}

func (p *parser) eof() bool {
	return p.pos >= len(p.in)
}

func (p *parser) skipOptionalWhitespace() {
	for !p.eof() && (p.in[p.pos] == ' ' || p.in[p.pos] == '\t') {
		p.pos++
	}
}

// bareItem reads a quoted string or run of bytes that could be a token, integer, decimal, or boolean.
func (p *parser) bareItem() (string, error) {
	if p.eof() {
		return "", fmt.Errorf("%w: unexpected end of input at offset %d", ErrParserSyntax, p.pos)
	}

	if p.in[p.pos] == '"' {
		return p.quotedString()
	}

	start := p.pos
	for !p.eof() {
		c := p.in[p.pos]
		// ensures runes are in the printable ascii range
		if c == ',' || c == ';' || c == '"' || c == ' ' || c == '\t' || c < 0x20 || c >= 0x7f {
			break
		}
		p.pos++
	}

	if p.pos == start {
		return "", fmt.Errorf("%w: %w starting at offset %d", ErrParserSyntax, ErrEmptyItem, start)
	}

	return p.in[start:p.pos], nil
}

// quotedString implements sf-string from RFC 8941 section 3.3.3.
//
// Caveat: only \" and \\ are legal escape sequences and the contents must be ASCII-printable.
func (p *parser) quotedString() (string, error) {
	open := p.pos
	p.pos++ // go past the opening quote

	var sb strings.Builder
	for !p.eof() {
		c := p.in[p.pos]
		p.pos++

		switch {
		case c == '"':
			return sb.String(), nil
		case c == '\\':
			if p.eof() {
				return "", fmt.Errorf("%w: %w at offset %d", ErrParserSyntax, ErrIllegalEscape, p.pos)
			}
			esc := p.in[p.pos]
			p.pos++
			if esc != '"' && esc != '\\' {
				return "", fmt.Errorf("%w: %w %q in string at offset %d", ErrParserSyntax, ErrIllegalEscape, esc, p.pos-1)
			}
			sb.WriteByte(esc)
		case c < 0x20 || c >= 0x7f: // less than space, greater than ~
			return "", fmt.Errorf("%w: %w at offset %d", ErrParserSyntax, ErrIllegalByte, p.pos-1)
		default:
			sb.WriteByte(c)
		}
	}

	return "", fmt.Errorf("%w: %w starting at offset %d", ErrParserSyntax, ErrUnterminatedString, open)
}

// params implements the parameters rule from RFC 8941 section 3.1.2.
//
// Bare parameters with no value are recorded as "?1" (true)
func (p *parser) params() (map[string]string, error) {
	result := map[string]string{}

	for !p.eof() && p.in[p.pos] == ';' {
		p.pos++
		p.skipOptionalWhitespace()

		key, err := p.paramKey()
		if err != nil {
			return nil, err
		}

		value := bareTrue
		if !p.eof() && p.in[p.pos] == '=' {
			p.pos++
			value, err = p.bareItem()
			if err != nil {
				return nil, err
			}
		}

		result[key] = value
	}

	return result, nil
}

// paramKey implements the parameter key rule from RFC 8941 section 3.1.2.
//
// The RFC says that keys can either start with lcalpha or "*", but in conformance with Postel's law,
// we're loosening the parser to allow lcalpha / DIGIT / "_" / "-" / "." / "*".
func (p *parser) paramKey() (string, error) {
	start := p.pos

	for !p.eof() {
		c := p.in[p.pos]
		lower := c >= 'a' && c <= 'z'
		digit := c >= '0' && c <= '9'
		if !lower && !digit && c != '_' && c != '-' && c != '.' && c != '*' {
			break
		}
		p.pos++
	}

	if p.pos == start {
		return "", fmt.Errorf("%w: %w starting at offset %d", ErrParserSyntax, ErrEmptyParameterKey, start)
	}

	return p.in[start:p.pos], nil
}
