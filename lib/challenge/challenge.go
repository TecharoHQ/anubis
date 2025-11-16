package challenge

import "time"

// Challenge is the metadata about a single challenge issuance.
type Challenge struct {
	IssuedAt       time.Time         `json:"issuedAt"`
	Metadata       map[string]string `json:"metadata"`
	ID             string            `json:"id"`
	Method         string            `json:"method"`
	RandomData     string            `json:"randomData"`
	PolicyRuleHash string            `json:"policyRuleHash,omitempty"`
	Difficulty     int               `json:"difficulty,omitempty"`
	Spent          bool              `json:"spent"`
}
