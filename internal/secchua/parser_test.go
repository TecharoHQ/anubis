package secchua

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCanonicalizeDoesntMutateInput(t *testing.T) {
	in := []Version{
		{"Not=A?Brand", "99"},
		{"Chromium", "151"},
	}

	want := append([]Version(nil), in...)

	Canonicalize(in)

	if diff := cmp.Diff(in, want); diff != "" {
		t.Errorf("input mutated unexpectedly (-want +got):\n%s", diff)
	}
}

func TestParse(t *testing.T) {
	for _, tt := range []struct {
		name, in        string
		err             error
		want, canonical []Version
	}{
		{
			name:      "empty header",
			in:        "    ",
			want:      nil,
			canonical: []Version{},
		},
		{
			name:      "illegal escape",
			in:        `"Foo\?";v="?0"`,
			err:       ErrIllegalEscape,
			want:      nil,
			canonical: nil,
		},
		{
			name:      "unterminated string",
			in:        `"Foo";v="`,
			err:       ErrUnterminatedString,
			want:      nil,
			canonical: nil,
		},
		{
			name:      "illegal byte",
			in:        fmt.Sprintf("\"Foo%c\";v=\"?1\"", 0x07),
			err:       ErrIllegalByte,
			want:      nil,
			canonical: nil,
		},
		{
			name:      "extra parameters ignored",
			in:        `chromium;v=151;wow="such param";flag`,
			want:      []Version{{Name: "chromium", Version: "151"}},
			canonical: []Version{{Name: "chromium", Version: "151"}},
		},
		{
			name:      "unquoted token and bare integer version",
			in:        `chromium;v=151`,
			want:      []Version{{Name: "chromium", Version: "151"}},
			canonical: []Version{{Name: "chromium", Version: "151"}},
		},
		{
			name:      "trailing comma",
			in:        `chromium;v="151", `,
			err:       ErrTrailingComma,
			want:      nil,
			canonical: nil,
		},
		{
			name:      "illegal end",
			in:        `chromium;v="151"q `,
			err:       ErrIllegalByte,
			want:      nil,
			canonical: nil,
		},
		{
			name:      "space before values", // no shipping browser does this, but Postel's law, etc
			in:        `chromium; v=151`,
			want:      []Version{{Name: "chromium", Version: "151"}},
			canonical: []Version{{Name: "chromium", Version: "151"}},
		},
		{
			name:      "Chrome 151",
			in:        `"Not=A?Brand";v="99", "Google Chrome";v="151", "Chromium";v="151"`,
			want:      []Version{{"Not=A?Brand", "99"}, {"Google Chrome", "151"}, {"Chromium", "151"}},
			canonical: []Version{{"Chromium", "151"}, {"Google Chrome", "151"}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.in)
			if !errors.Is(err, tt.err) {
				t.Logf("want: %v", tt.err)
				t.Logf(" got: %v", err)
				t.Error("unexpected error")
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Parse(%q) (-want +got):\n%s", tt.in, diff)
			}

			canon, err := ParseCanonical(tt.in)
			if !errors.Is(err, tt.err) {
				t.Logf("want: %v", tt.err)
				t.Logf(" got: %v", err)
				t.Error("unexpected error")
			}

			if diff := cmp.Diff(tt.canonical, canon); diff != "" {
				t.Errorf("Parse(%q) (-want +got):\n%s", tt.in, diff)
			}
		})
	}
}

func FuzzParse(f *testing.F) {
	f.Fuzz(func(t *testing.T, header string) {
		vs, err := Parse(header)
		if err != nil {
			return
		}
		// Canonicalize must never panic and must never grow the list.
		if got := Canonicalize(vs); len(got) > len(vs) {
			t.Fatalf("Canonicalize grew %d -> %d", len(vs), len(got))
		}
	})
}
