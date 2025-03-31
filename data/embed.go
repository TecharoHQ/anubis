package data

import "embed"

var (
	//go:embed botPolicies.json
	BotPolicies embed.FS

	//go:embed translations.yaml
	Translations embed.FS
)
