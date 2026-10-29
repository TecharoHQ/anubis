package policy

import (
	"net/http"
	"testing"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/data"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/geoip/geoiptest"
)

func BenchmarkDefaultPolicyBots(b *testing.B) {
	fin, err := data.BotPolicies.Open("botPolicies.yaml")
	if err != nil {
		b.Fatal(err)
	}
	defer fin.Close() //nolint:errcheck

	pc, err := ParseConfig(geoiptest.WithMockGeoIP(b), fin, "botPolicies.yaml", anubis.DefaultDifficulty, "info", false)
	if err != nil {
		b.Fatalf("can't parse config: %v", err)
	}

	r, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		b.Fatalf("can't make request: %v", err)
	}
	r.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0")
	r.Header.Set("X-Real-IP", "127.0.0.1")

	for b.Loop() {
		for i := range pc.Bots {
			bot := &pc.Bots[i]
			ok, err := bot.Rules.Check(r)
			if err != nil {
				b.Fatal(err)
			}

			if ok && bot.Action != config.RuleWeigh {
				break
			}
		}
	}
}
