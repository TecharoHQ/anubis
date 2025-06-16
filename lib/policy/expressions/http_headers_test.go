package expressions

import (
	"net/http"
	"testing"

	"github.com/google/cel-go/common/types"
)

func TestHTTPHeaders(t *testing.T) {
	headers := HTTPHeaders{
		Header: http.Header{
			"Content-Type": {"application/json"},
			"Cf-Worker":    {"true"},
			"User-Agent":   {"Go-http-client/2"},
		},
	}

	t.Run("contains-existing-header", func(t *testing.T) {
		resp := headers.Contains(types.String("User-Agent"))
		if !resp.(types.Bool) {
			t.Fatal("headers does not contain User-Agent")
		}
	})

	t.Run("not-contains-missing-header", func(t *testing.T) {
		resp := headers.Contains(types.String("Xxx-Random-Header"))
		if resp.(types.Bool) {
			t.Fatal("headers does not contain User-Agent")
		}
	})

	t.Run("get-existing-header", func(t *testing.T) {
		val := headers.Get(types.String("User-Agent"))
		switch val.(type) {
		case types.String:
			// ok
		default:
			t.Fatalf("result was wrong type %T", val)
		}
	})

	t.Run("not-get-missing-header", func(t *testing.T) {
		val := headers.Get(types.String("Xxx-Random-Header"))
		switch val.(type) {
		case *types.Err:
			// ok
		default:
			t.Fatalf("result was wrong type %T", val)
		}
	})
}
