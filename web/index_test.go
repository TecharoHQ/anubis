package web

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/localization"
	"github.com/a-h/templ"
)

func TestBasePrefixInLinks(t *testing.T) {
	tests := []struct {
		name       string
		basePrefix string
		wantInLink string
	}{
		{
			name:       "no prefix",
			basePrefix: "",
			wantInLink: "/.within.website/x/cmd/anubis/api/",
		},
		{
			name:       "with rififi prefix",
			basePrefix: "/rififi",
			wantInLink: "/rififi/.within.website/x/cmd/anubis/api/",
		},
		{
			name:       "with myapp prefix",
			basePrefix: "/myapp",
			wantInLink: "/myapp/.within.website/x/cmd/anubis/api/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original BasePrefix and restore after test
			origPrefix := anubis.BasePrefix
			defer func() { anubis.BasePrefix = origPrefix }()

			anubis.BasePrefix = tt.basePrefix

			// Create test impressum
			impressum := &config.Impressum{
				Footer: "<p>Test footer</p>",
				Page: config.ImpressumPage{
					Title: "Test Imprint",
					Body:  "<p>Test imprint body</p>",
				},
			}

			// Create localizer using a dummy request
			req := httptest.NewRequest("GET", "/", nil)
			localizer := &localization.SimpleLocalizer{}
			localizer.Localizer = localization.NewLocalizationService().GetLocalizerFromRequest(req)

			// Render the base template to a buffer
			var buf strings.Builder
			component := base(tt.name, templ.NopComponent, impressum, nil, nil, localizer)
			err := component.Render(context.Background(), &buf)
			if err != nil {
				t.Fatalf("failed to render template: %v", err)
			}

			output := buf.String()

			// Check that honeypot link includes the base prefix
			if !strings.Contains(output, `href="`+tt.wantInLink+`honeypot/`) {
				t.Errorf("honeypot link does not contain base prefix %q\noutput: %s", tt.wantInLink, output)
			}

			// Check that imprint link includes the base prefix
			if !strings.Contains(output, `href="`+tt.wantInLink+`imprint`) {
				t.Errorf("imprint link does not contain base prefix %q\noutput: %s", tt.wantInLink, output)
			}
		})
	}
}
