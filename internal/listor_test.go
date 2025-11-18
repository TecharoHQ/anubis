package internal

import (
	"encoding/json"
	"testing"
)

func TestListOr_UnmarshalJSON(t *testing.T) {
	t.Run("single value should be unmarshaled as single item", func(t *testing.T) {
		var lo ListOr[string]

		err := json.Unmarshal([]byte(`"hello"`), &lo)
		if err != nil {
			t.Fatalf("Failed to unmarshal single string: %v", err)
		}

		if len(lo) != 1 {
			t.Fatalf("Expected 1 item, got %d", len(lo))
		}

		if lo[0] != "hello" {
			t.Errorf("Expected 'hello', got %q", lo[0])
		}
	})

	t.Run("array should be unmarshaled as multiple items", func(t *testing.T) {
		var lo ListOr[string]

		err := json.Unmarshal([]byte(`["hello", "world"]`), &lo)
		if err != nil {
			t.Fatalf("Failed to unmarshal array: %v", err)
		}

		if len(lo) != 2 {
			t.Fatalf("Expected 2 items, got %d", len(lo))
		}

		if lo[0] != "hello" {
			t.Errorf("Expected 'hello', got %q", lo[0])
		}
		if lo[1] != "world" {
			t.Errorf("Expected 'world', got %q", lo[1])
		}
	})

	t.Run("single number should be unmarshaled as single item", func(t *testing.T) {
		var lo ListOr[int]

		err := json.Unmarshal([]byte(`42`), &lo)
		if err != nil {
			t.Fatalf("Failed to unmarshal single number: %v", err)
		}

		if len(lo) != 1 {
			t.Fatalf("Expected 1 item, got %d", len(lo))
		}

		if lo[0] != 42 {
			t.Errorf("Expected 42, got %d", lo[0])
		}
	})

	t.Run("array of numbers should be unmarshaled as multiple items", func(t *testing.T) {
		var lo ListOr[int]

		err := json.Unmarshal([]byte(`[1, 2, 3]`), &lo)
		if err != nil {
			t.Fatalf("Failed to unmarshal number array: %v", err)
		}

		if len(lo) != 3 {
			t.Fatalf("Expected 3 items, got %d", len(lo))
		}

		if lo[0] != 1 || lo[1] != 2 || lo[2] != 3 {
			t.Errorf("Expected [1, 2, 3], got %v", lo)
		}
	})
}