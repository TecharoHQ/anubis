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
			component := base(tt.name, templ.NopComponent, impressum, &config.Honeypot{Enabled: true, Implementation: "naive"}, nil, nil, localizer)
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

func TestHoneypotLinkEscapesHref(t *testing.T) {
	var out strings.Builder
	if err := honeypotLink(`/path" onclick="bad"><script>bad</script>`).Render(t.Context(), &out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), `href="/path" onclick=`) || strings.Contains(out.String(), "<script>bad") {
		t.Fatalf("unsafe output: %s", out.String())
	}
}

func TestLocalizedTemplateStrings(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Language", "pt-PT")
	localizer := &localization.SimpleLocalizer{
		Localizer: localization.NewLocalizationService().GetLocalizerFromRequest(req),
	}

	impressum := &config.Impressum{
		Footer: "<p>Test footer</p>",
		Page: config.ImpressumPage{
			Title: "Test Imprint",
			Body:  "<p>Test imprint body</p>",
		},
	}

	var baseOut strings.Builder
	if err := base("test", templ.NopComponent, impressum, nil, nil, nil, localizer).Render(t.Context(), &baseOut); err != nil {
		t.Fatal(err)
	}
	// cspell:ignore Informação
	if !strings.Contains(baseOut.String(), ">Informação legal</a>") {
		t.Fatalf("localized imprint label missing: %s", baseOut.String())
	}

	var errorOut strings.Builder
	if err := errorPage("test", "", "", localizer).Render(t.Context(), &errorOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errorOut.String(), `alt="Anubis triste"`) {
		t.Fatalf("localized error image alt text missing: %s", errorOut.String())
	}
}
