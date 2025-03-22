package policy

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/TecharoHQ/anubis/lib/policy/config"
)

var (
	PolicyApplications = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "anubis_policy_results",
		Help: "The results of each policy rule",
	}, []string{"rule", "action"})
)

type ParsedConfig struct {
	orig config.Config

	Bots  []Bot
	DNSBL bool
}

func NewParsedConfig(orig config.Config) *ParsedConfig {
	return &ParsedConfig{
		orig: orig,
	}
}
