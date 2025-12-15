package honeypot

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var Timings = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "anubis",
	Subsystem: "honeypot",
	Name:      "pagegen_timings",
	Help:      "The amount of time honeypot page generation takes per method",
	Buckets:   prometheus.ExponentialBuckets(0.5, 2, 32),
}, []string{"method"})

type Info struct {
	CreatedAt time.Time `json:"createdAt"`
	UserAgent string    `json:"userAgent"`
	IPAddress string    `json:"ipAddress"`
	HitCount  int       `json:"hitCount"`
}
