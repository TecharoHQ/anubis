package expressions

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
)

type registryResolver struct{ r *http.Request }

func (rr registryResolver) ResolveName(name string) (any, bool) {
	return ResolveBotVariable(name, rr.r)
}

func (ra registryResolver) Parent() cel.Activation { return nil }

func TestRegisterBotVariable(t *testing.T) {
	RegisterBotVariable("registryTestScore", cel.IntType, func(r *http.Request) (any, bool) {
		sc := r.Header.Get("X-Score")
		if sc == "" {
			return nil, false
		}

		result, err := strconv.Atoi(sc)
		if err != nil {
			panic("score is not a number")
		}

		return result, true
	})

	env, err := BotEnvironment(newTestDNS(300, 300))
	if err != nil {
		t.Fatalf("can't create bot environment: %v", err)
	}

	t.Run("variable exists in compile", func(t *testing.T) {
		if _, err := Compile(env, `registryTestScore >= 3`); err != nil {
			t.Fatalf("cannot compile environment with added variable (is it not an int?): %v", err)
		}
	})

	t.Run("variable is not a string", func(t *testing.T) {
		if _, err := Compile(env, `registryTestScore == "taco bell"`); err == nil {
			t.Fatalf("cannot compile program treating this as a string (is the type wrong?)")
		}
	})

	for _, tt := range []struct {
		name    string
		score   int
		want    bool
		wantErr bool
	}{
		{"matches", 3, true, false},
		{"does not match", 2, false, false},
		{"no value", 0, false, true},
	} {
		t.Run("case/"+tt.name, func(t *testing.T) {
			const src = `registryTestScore >= 3`
			prog, err := Compile(env, src)
			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			if tt.score != 0 {
				r.Header.Set("X-Score", fmt.Sprint(tt.score))
			}

			got, _, err := prog.ContextEval(t.Context(), registryResolver{r})
			if (err != nil) != tt.wantErr {
				t.Errorf("wanted error: %v, got error: %v", tt.wantErr, err)
			}
			if tt.wantErr {
				return
			}

			if got != types.Bool(tt.want) {
				t.Errorf("wanted %v, got: %v", tt.want, got)
			}
		})
	}

	t.Run("duplicate reg panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("second registration did not panic")
			}
		}()
		RegisterBotVariable("registryTestScore", cel.IntType, nil)
	})

	t.Run("unknown variable does not resolve", func(t *testing.T) {
		if _, ok := ResolveBotVariable("registryTestMissing", httptest.NewRequest(http.MethodGet, "/", nil)); ok {
			t.Error("unregistered variable resolved")
		}
	})
}
