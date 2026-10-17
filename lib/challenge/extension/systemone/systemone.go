package systemone

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/challenge/extension"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/policy"
	"github.com/TecharoHQ/anubis/lib/store"
	"github.com/a-h/templ"
)

func init() {
	extension.Register("systemone", &Impl{})
}

type Impl struct {
	cli    *Client
	model  string
	sysOne *config.SystemOne
}

func (i *Impl) Setup(mux *http.ServeMux, st store.Interface, cfg *policy.ParsedConfig) error {
	if cfg.SystemOne == nil {
		return fmt.Errorf("extension/systemone: you must define a `systemone:` block in your configuration to use this extension")
	}

	i.cli = NewClient(cfg.SystemOne.URL)
	i.model = cfg.SystemOne.Model
	i.sysOne = cfg.SystemOne

	apiKey, ok := os.LookupEnv(cfg.SystemOne.APIKeyEnv)
	if !ok {
		return fmt.Errorf("extension/systemone: API key envvar not defined: %q (hint: this must be an environment variable, not the API key itself)", cfg.SystemOne.APIKeyEnv)
	}

	i.cli.APIKey = apiKey

	return nil
}

func (i *Impl) Head(r *http.Request, chall *challenge.Challenge) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		return nil
	})
}

func (i *Impl) Validate(r *http.Request, lg *slog.Logger, in *challenge.ValidateInput) error {
	clientContext := map[string]any{
		"userAgent":      r.UserAgent(),
		"accept":         r.Header.Get("Accept"),
		"acceptEncoding": r.Header.Get("Accept-Encoding"),
		"acceptLanguage": r.Header.Get("Accept-Language"),
		"secFetch": map[string]any{
			"dest": r.Header.Get("Sec-Fetch-Dest"),
			"mode": r.Header.Get("Sec-Fetch-Mode"),
			"site": r.Header.Get("Sec-Fetch-Site"),
		},
		"targetURL": in.Challenge.TargetURL,
	}

	if domainContext, ok := i.sysOne.Context[r.Host]; ok {
		clientContext["domainContext"] = domainContext
	}

	gip, ok := lookupGeoIP(r)
	if ok {
		lg.DebugContext(r.Context(), "found geoip data", "geoip", gip)
		clientContext["geoip"] = gip
	}

	jevReq := &Request{
		Model: i.model,
		State: clientContext,
		Questions: map[string]Question{
			"legit": {
				Type:         Noul,
				Instructions: "Is this client legitimate?",
			},
			"language": {
				Type:         Noul,
				Instructions: "Does the client language match the country?",
			},
			"deep_link": {
				Type:         Noul,
				Instructions: "Is the target URL a deep link?",
			},
		},
	}
	jevResp, err := i.cli.Evaluate(r.Context(), jevReq)
	if err != nil {
		lg.ErrorContext(r.Context(), "error contacting system one API, failing open", "err", err)
		return nil
	}

	var score = 0
	var scores map[string]float64
	for name, answer := range jevResp.Answers {
		switch name {
		case "legit":
			if answer.Noul != nil && *answer.Noul >= 0.8 {
				score += 5
			}
		case "language":
			if answer.Noul != nil && *answer.Noul >= 0.6 {
				score += 2
			}
		case "deep_link":
			if answer.Noul != nil && *answer.Noul >= 0.9 {
				score += 5
			}
		}
	}

	if score >= 6 {
		return challenge.NewError("extension/systemone", "Please adjust the vibes of your client and try your request again", fmt.Errorf("jev said: legit: %v, language: %v, deep_link: %v", scores["legit"], scores["language"], scores["deep_link"]))
	}

	return nil
}
