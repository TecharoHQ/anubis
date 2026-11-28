package challenge

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// MaxElapsedTime is the maximum client-reported elapsedTime (in milliseconds)
// accepted by ParseElapsedTime. Issued challenges expire after 30 minutes, so
// values above this bound cannot represent a valid solve duration and could
// skew or overflow the TimeTaken histogram sum.
const MaxElapsedTime = float64(30 * time.Minute / time.Millisecond)

var TimeTaken = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "anubis_time_taken",
	Help:    "The time taken for a browser to generate a response (milliseconds)",
	Buckets: prometheus.ExponentialBucketsRange(1, math.Pow(2, 20), 20),
}, []string{"method"})

// ParseElapsedTime parses the client-reported elapsedTime form value. The value
// is client controlled and is fed into the TimeTaken histogram, so NaN,
// infinite, negative, and excessively large values are rejected:
// strconv.ParseFloat accepts "NaN", "Inf", and large finite floats such as
// "1e308" whose observations would permanently poison or overflow the
// histogram's sum.
func ParseElapsedTime(raw string) (float64, error) {
	elapsedTime, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(elapsedTime) || math.IsInf(elapsedTime, 0) || elapsedTime < 0 || elapsedTime > MaxElapsedTime {
		return 0, fmt.Errorf("elapsed time %q must be a finite number between 0 and %g", raw, MaxElapsedTime)
	}
	return elapsedTime, nil
}
