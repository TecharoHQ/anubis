package expressions

import (
	"errors"
	"testing"

	"github.com/google/cel-go/cel"
)

func TestJoin(t *testing.T) {
	env, err := NewEnvironment()
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		err       error
		name      string
		op        JoinOperator
		resultStr string
		clauses   []string
	}{
		{
			name:    "no-clauses",
			clauses: []string{},
			op:      JoinAnd,
			err:     ErrNoExpressions,
		},
		{
			name:      "one-clause-identity",
			clauses:   []string{`remoteAddress == "8.8.8.8"`},
			op:        JoinAnd,
			err:       nil,
			resultStr: `remoteAddress == "8.8.8.8"`,
		},
		{
			name: "multi-clause-and",
			clauses: []string{
				`remoteAddress == "8.8.8.8"`,
				`host == "anubis.techaro.lol"`,
			},
			op:        JoinAnd,
			err:       nil,
			resultStr: `remoteAddress == "8.8.8.8" && host == "anubis.techaro.lol"`,
		},
		{
			name: "multi-clause-or",
			clauses: []string{
				`remoteAddress == "8.8.8.8"`,
				`host == "anubis.techaro.lol"`,
			},
			op:        JoinOr,
			err:       nil,
			resultStr: `remoteAddress == "8.8.8.8" || host == "anubis.techaro.lol"`,
		},
		{
			name: "git-user-agent",
			clauses: []string{
				`userAgent.startsWith("git/") || userAgent.contains("libgit")`,
				`"Git-Protocol" in headers && headers["Git-Protocol"] == "version=2"`,
			},
			op:  JoinAnd,
			err: nil,
			resultStr: `(userAgent.startsWith("git/") || userAgent.contains("libgit")) && "Git-Protocol" in headers &&
headers["Git-Protocol"] == "version=2"`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Join(env, tt.op, tt.clauses...)
			if !errors.Is(err, tt.err) {
				t.Errorf("wanted error %v but got: %v", tt.err, err)
			}

			if tt.err != nil {
				return
			}

			program, err := cel.AstToString(result)
			if err != nil {
				t.Fatalf("can't decompile program: %v", err)
			}

			if tt.resultStr != program {
				t.Logf("wanted: %s", tt.resultStr)
				t.Logf("got: %s", program)
				t.Error("program did not compile as expected")
			}
		})
	}
}
