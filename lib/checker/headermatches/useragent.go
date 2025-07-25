package headermatches

import (
	"context"
	"encoding/json"

	"github.com/TecharoHQ/anubis/lib/checker"
)

func ValidUserAgent(valueRex string) error {
	fc := fileConfig{
		Header:     "User-Agent",
		ValueRegex: valueRex,
	}

	return fc.Valid()
}

func NewUserAgent(valueRex string) (checker.Interface, error) {
	fc := fileConfig{
		Header:     "User-Agent",
		ValueRegex: valueRex,
	}

	if err := fc.Valid(); err != nil {
		return nil, err
	}

	data, err := json.Marshal(fc)
	if err != nil {
		return nil, err
	}

	return Factory{}.Build(context.Background(), json.RawMessage(data))
}
