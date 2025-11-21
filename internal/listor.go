package internal

import (
	"encoding/json"
)

// ListOr[T any] is a slice that can contain either a single T or multiple T values.
// During JSON unmarshaling, it checks if the first character is '[' to determine
// whether to treat the JSON as an array or a single value.
type ListOr[T any] []T

func (lo *ListOr[T]) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	// Check if first non-whitespace character is '['
	firstChar := data[0]
	for i := 0; i < len(data); i++ {
		if data[i] != ' ' && data[i] != '\t' && data[i] != '\n' && data[i] != '\r' {
			firstChar = data[i]
			break
		}
	}

	if firstChar == '[' {
		// It's an array, unmarshal directly
		return json.Unmarshal(data, (*[]T)(lo))
	} else {
		// It's a single value, unmarshal as a single item in a slice
		var single T
		if err := json.Unmarshal(data, &single); err != nil {
			return err
		}
		*lo = ListOr[T]{single}
	}

	return nil
}