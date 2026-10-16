package geoip

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	databaseBuildTime = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "anubis_geoip_database_build_timestamp_seconds",
		Help: "Build time of the loaded geoip database as a Unix timestamp",
	}, []string{"kind"})

	updateErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "anubis_geoip_update_errors_total",
		Help: "Number of failed geoip database updates or reloads",
	}, []string{"kind"})
)
