package data

import "embed"

var (
	//go:embed botPolicies.yaml botPolicies.json all:apps all:bots all:clients all:common all:crawlers all:meta
	BotPolicies embed.FS
)
