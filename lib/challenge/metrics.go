package challenge

import (
	"math"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var TimeTaken = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "anubis_time_taken",
	Help:    "The time taken for a browser to generate a response (milliseconds)",
	Buckets: prometheus.ExponentialBucketsRange(1, math.Pow(2, 20), 20),
}, []string{"method"})
