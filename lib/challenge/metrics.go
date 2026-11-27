package challenge

import (
	"fmt"
	"math"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var TimeTaken = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "anubis_time_taken",
	Help:    "The time taken for a browser to generate a response (milliseconds)",
	Buckets: prometheus.ExponentialBucketsRange(1, math.Pow(2, 20), 20),
}, []string{"method"})

// ParseElapsedTime parses the client-reported elapsedTime form value. The value
// is client controlled and is fed into the TimeTaken histogram, so NaN, infinite
// and negative values are rejected: strconv.ParseFloat accepts "NaN" and "Inf",
// and observing them would permanently poison the histogram's sum.
func ParseElapsedTime(raw string) (float64, error) {
	elapsedTime, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(elapsedTime) || math.IsInf(elapsedTime, 0) || elapsedTime < 0 {
		return 0, fmt.Errorf("elapsed time %q is not a finite non-negative number", raw)
	}
	return elapsedTime, nil
}
