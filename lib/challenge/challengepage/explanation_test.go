package challengepage

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TecharoHQ/anubis/lib/localization"
)

func TestExplanationHTML(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain text",
			in:   "hello & goodbye",
			want: "hello &amp; goodbye",
		},
		{
			name: "wikipedia link",
			in:   `called <a href="https://en.wikipedia.org/wiki/Proof_of_work">Proof of Work</a>.`,
			want: `called <a href="https://en.wikipedia.org/wiki/Proof_of_work">Proof of Work</a>.`,
		},
		{
			name: "localized wikipedia link",
			in:   `<a href="https://de.wikipedia.org/wiki/Proof_of_Work">Proof of Work</a>`,
			want: `<a href="https://de.wikipedia.org/wiki/Proof_of_Work">Proof of Work</a>`,
		},
		{
			name: "script tag",
			in:   `<script>alert(1)</script>`,
			want: `&lt;script&gt;alert(1)&lt;/script&gt;`,
		},
		{
			name: "javascript href",
			in:   `<a href="javascript:alert(1)">x</a>`,
			want: `&lt;a href=&#34;javascript:alert(1)&#34;&gt;x&lt;/a&gt;`,
		},
		{
			name: "attribute breakout",
			in:   `<a href="https://en.wikipedia.org/wiki/X" onclick="alert(1)">x</a>`,
			want: `&lt;a href=&#34;https://en.wikipedia.org/wiki/X&#34; onclick=&#34;alert(1)&#34;&gt;x&lt;/a&gt;`,
		},
		{
			name: "markup inside link text",
			in:   `<a href="https://en.wikipedia.org/wiki/X"><img src=x onerror=alert(1)></a>`,
			want: `<a href="https://en.wikipedia.org/wiki/X">&lt;img src=x onerror=alert(1)&gt;</a>`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := explanationHTML(tt.in); got != tt.want {
				t.Errorf("explanationHTML(%q)\n got: %s\nwant: %s", tt.in, got, tt.want)
			}
		})
	}
}

// TestExplanationRendersLink makes sure no shipped translation of
// simplified_explanation renders its Proof of Work link as escaped text.
func TestExplanationRendersLink(t *testing.T) {
	files, err := filepath.Glob("../../localization/locales/*.json")
	if err != nil {
		t.Fatal(err)
	}

	for _, fname := range files {
		lang := strings.TrimSuffix(filepath.Base(fname), ".json")
		if lang == "manifest" {
			continue
		}

		t.Run(lang, func(t *testing.T) {
			data, err := os.ReadFile(fname)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), "wikipedia.org") {
				t.Skip("translation has no link")
			}

			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("Accept-Language", lang)

			var sb strings.Builder
			if err := Explanation(localization.GetLocalizer(req)).Render(t.Context(), &sb); err != nil {
				t.Fatal(err)
			}
			out := sb.String()

			if strings.Contains(out, "&lt;a") || strings.Contains(out, "&lt;/a") {
				t.Errorf("rendered explanation contains escaped anchor markup: %s", out)
			}
			if !strings.Contains(out, `<a href="https://`) {
				t.Errorf("rendered explanation has no link: %s", out)
			}
		})
	}
}
