package data

import "embed"

var (
	//go:embed botPolicies.yaml all:apps all:bots all:clients all:common all:crawlers all:meta all:security all:services
	BotPolicies embed.FS
)
