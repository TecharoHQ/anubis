package challenge

import "time"

// Challenge is the metadata about a single challenge issuance.
type Challenge struct {
	ID             string            `json:"id"`                       // UUID identifying the challenge
	Method         string            `json:"method"`                   // Challenge method
	RandomData     string            `json:"randomData"`               // The random data the client processes
	IssuedAt       time.Time         `json:"issuedAt"`                 // When the challenge was issued
	Metadata       map[string]string `json:"metadata"`                 // Challenge metadata such as IP address and user agent
	Spent          bool              `json:"spent"`                    // Has the challenge already been solved?
	Difficulty     int               `json:"difficulty,omitempty"`     // Difficulty that was in effect when issued
	PolicyRuleHash string            `json:"policyRuleHash,omitempty"` // Hash of the policy rule that issued this challenge
}
