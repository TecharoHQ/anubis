package data

import "embed"

var (
	//go:embed botPolicies.json
	BotPolicies embed.FS
)
