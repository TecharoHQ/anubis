package expressions

import (
	"net/url"
	"testing"

	"github.com/google/cel-go/common/types"
)

func TestURLValues(t *testing.T) {
	headers := URLValues{
		Values: url.Values{
			"format": {"json"},
		},
	}

	t.Run("contains-existing-key", func(t *testing.T) {
		resp := headers.Contains(types.String("format"))
		if !resp.(types.Bool) {
			t.Fatal("headers does not contain User-Agent")
		}
	})

	t.Run("not-contains-missing-key", func(t *testing.T) {
		resp := headers.Contains(types.String("not-there"))
		if resp.(types.Bool) {
			t.Fatal("headers does not contain User-Agent")
		}
	})

	t.Run("get-existing-key", func(t *testing.T) {
		val := headers.Get(types.String("format"))
		switch val.(type) {
		case types.String:
			// ok
		default:
			t.Fatalf("result was wrong type %T", val)
		}
	})

	t.Run("not-get-missing-key", func(t *testing.T) {
		val := headers.Get(types.String("not-there"))
		switch val.(type) {
		case *types.Err:
			// ok
		default:
			t.Fatalf("result was wrong type %T", val)
		}
	})
}
