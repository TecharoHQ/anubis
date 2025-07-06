package challenge

import "time"

// Challenge is the metadata about a single challenge issuance.
type Challenge struct {
	ID         string            `json:"id"`         // UUID identifying the challenge
	RandomData string            `json:"randomData"` // The random data the client processes
	IssuedAt   time.Time         `json:"issuedAt"`   // When the challenge was issued
	Metadata   map[string]string `json:"metadata"`   // Challenge metadata such as IP address and user agent
}
