package config

import (
	"bytes"
	"errors"
	"testing"
)

func TestImpressumValid(t *testing.T) {
	for _, cs := range []struct {
		err  error
		inp  Impressum
		name string
	}{
		{
			name: "basic happy path",
			inp: Impressum{
				Footer: "<p>Website hosted by Techaro.<p>",
				Page: ImpressumPage{
					Title: "Techaro Imprint",
					Body:  "<p>This is an imprint page.</p>",
				},
			},
			err: nil,
		},
		{
			name: "no footer",
			inp: Impressum{
				Footer: "",
				Page: ImpressumPage{
					Title: "Techaro Imprint",
					Body:  "<p>This is an imprint page.</p>",
				},
			},
			err: ErrMissingValue,
		},
		{
			name: "page not valid",
			inp: Impressum{
				Footer: "test page please ignore",
			},
			err: ErrMissingValue,
		},
	} {
		t.Run(cs.name, func(t *testing.T) {
			if err := cs.inp.Valid(); !errors.Is(err, cs.err) {
				t.Logf("want: %v", cs.err)
				t.Logf("got:  %v", err)
				t.Error("validation failed")
			}

			var buf bytes.Buffer
			if err := cs.inp.Render(t.Context(), &buf); err != nil {
				t.Errorf("can't render footer: %v", err)
			}

			if err := cs.inp.Page.Render(t.Context(), &buf); err != nil {
				t.Errorf("can't render page: %v", err)
			}
		})
	}
}
