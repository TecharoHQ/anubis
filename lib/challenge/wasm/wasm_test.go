package wasm

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"testing"

	"github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/policy"
	"github.com/TecharoHQ/anubis/wasm"
	"github.com/TecharoHQ/anubis/web"
)

// difficulty is intentionally tiny so that solving the proof of work in tests
// is effectively instant.
const difficulty = 4

// mkRequest builds a GET request whose query parameters become the form values
// read by Validate via r.FormValue.
func mkRequest(t *testing.T, values map[string]string) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	q := req.URL.Query()
	for k, v := range values {
		q.Set(k, v)
	}
	req.URL.RawQuery = q.Encode()

	return req
}

// solve runs the same WASM module the server uses to compute a genuinely valid
// (nonce, response) pair for the given challenge data, so the happy-path case
// exercises the full verification flow rather than a hand-rolled constant.
func solve(t *testing.T, algorithm string, data []byte) (nonce string, response string) {
	t.Helper()

	fname := fmt.Sprintf("static/wasm/simd128/%s.wasm", algorithm)
	fin, err := web.Static.Open(fname)
	if err != nil {
		t.Fatalf("can't open %s: %v", fname, err)
	}
	defer fin.Close()

	runner, err := wasm.NewRunner(t.Context(), fname, fin)
	if err != nil {
		t.Fatalf("can't create runner for %s: %v", algorithm, err)
	}

	n, hash, err := runner.Run(t.Context(), data, difficulty, 0, 1)
	if err != nil {
		t.Fatalf("can't compute valid solution: %v", err)
	}

	return strconv.Itoa(int(n)), hex.EncodeToString(hash)
}

// TestValidateAdversarial feeds Validate a matrix of malformed and hostile
// inputs. Several of these cases are regression guards:
//
//   - the "nil-*" cases reproduce the #1463-class nil-pointer panic that occurs
//     when Validate dereferences in.Challenge / in.Rule.Challenge without first
//     calling in.Valid().
//   - "response-wrong-length" reproduces the nil api.Module panic in
//     Runner.Verify when writeVerification rejects a correctly-encoded but
//     wrong-sized response.
//
// None of these inputs may panic, and each must surface a clean error.
func TestValidateAdversarial(t *testing.T) {
	const algorithm = "sha256"

	i := &Impl{algorithm: algorithm}
	if err := i.Setup(http.NewServeMux()); err != nil {
		t.Fatalf("can't set up %s challenge: %v", algorithm, err)
	}

	lg := slog.New(slog.NewTextHandler(io.Discard, nil))

	// A challenge whose RandomData is valid hex-encoded bytes.
	challengeBytes := sha256.Sum256([]byte(t.Name()))
	randomData := hex.EncodeToString(challengeBytes[:])

	validNonce, validResponse := solve(t, algorithm, challengeBytes[:])

	bot := &policy.Bot{
		Challenge: &config.ChallengeRules{
			Algorithm:  algorithm,
			Difficulty: difficulty,
		},
	}

	// goodInput returns a fresh, fully-populated ValidateInput so individual
	// cases can override only what they want to break.
	goodInput := func() *challenge.ValidateInput {
		return &challenge.ValidateInput{
			Rule:      bot,
			Challenge: &challenge.Challenge{RandomData: randomData},
		}
	}

	// goodValues returns a fresh set of form values for a valid solution.
	goodValues := func() map[string]string {
		return map[string]string{
			"nonce":       validNonce,
			"elapsedTime": "42",
			"response":    validResponse,
		}
	}

	// errNotNil asserts only that an error was returned (used where the failure
	// path does not wrap a package sentinel).
	errNotNil := func(err error) bool { return err != nil }

	tests := []struct {
		name     string
		values   map[string]string
		input    *challenge.ValidateInput
		wantErr  error                  // matched with errors.Is; nil means expect success
		errCheck func(err error) bool   // optional override; when set, wantErr is ignored
	}{
		{
			name:    "valid-solution-passes",
			values:  goodValues(),
			input:   goodInput(),
			wantErr: nil,
		},
		{
			name:    "no-fields",
			values:  map[string]string{},
			input:   goodInput(),
			wantErr: challenge.ErrMissingField,
		},
		{
			name: "missing-nonce",
			values: map[string]string{
				"elapsedTime": "42",
				"response":    validResponse,
			},
			input:   goodInput(),
			wantErr: challenge.ErrMissingField,
		},
		{
			name: "missing-elapsedTime",
			values: map[string]string{
				"nonce":    validNonce,
				"response": validResponse,
			},
			input:   goodInput(),
			wantErr: challenge.ErrMissingField,
		},
		{
			name: "missing-response",
			values: map[string]string{
				"nonce":       validNonce,
				"elapsedTime": "42",
			},
			input:   goodInput(),
			wantErr: challenge.ErrMissingField,
		},
		{
			name: "nonce-not-an-integer",
			values: map[string]string{
				"nonce":       "taco",
				"elapsedTime": "42",
				"response":    validResponse,
			},
			input:   goodInput(),
			wantErr: challenge.ErrInvalidFormat,
		},
		{
			name: "nonce-overflows-int",
			values: map[string]string{
				"nonce":       "999999999999999999999999999999",
				"elapsedTime": "42",
				"response":    validResponse,
			},
			input:   goodInput(),
			wantErr: challenge.ErrInvalidFormat,
		},
		{
			name: "elapsedTime-not-a-number",
			values: map[string]string{
				"nonce":       validNonce,
				"elapsedTime": "taco",
				"response":    validResponse,
			},
			input:   goodInput(),
			wantErr: challenge.ErrInvalidFormat,
		},
		{
			name: "response-not-hex",
			values: map[string]string{
				"nonce":       validNonce,
				"elapsedTime": "42",
				"response":    "zz",
			},
			input:   goodInput(),
			wantErr: challenge.ErrInvalidFormat,
		},
		{
			// Correctly hex-encoded but the wrong number of bytes. This drove a
			// nil api.Module dereference panic in Runner.Verify.
			name: "response-wrong-length",
			values: map[string]string{
				"nonce":       validNonce,
				"elapsedTime": "42",
				"response":    "abcd",
			},
			input:    goodInput(),
			errCheck: errNotNil,
		},
		{
			// Correct length, valid hex, but not the hash we asked for.
			name: "response-wrong-hash",
			values: map[string]string{
				"nonce":       "0",
				"elapsedTime": "42",
				"response":    hex.EncodeToString(make([]byte, sha256.Size)),
			},
			input:   goodInput(),
			wantErr: challenge.ErrFailed,
		},
		{
			name:    "nil-input",
			values:  goodValues(),
			input:   nil,
			wantErr: challenge.ErrInvalidInput,
		},
		{
			name:    "nil-rule",
			values:  goodValues(),
			input:   &challenge.ValidateInput{Challenge: &challenge.Challenge{RandomData: randomData}},
			wantErr: challenge.ErrInvalidInput,
		},
		{
			name:    "nil-rule-challenge",
			values:  goodValues(),
			input:   &challenge.ValidateInput{Rule: &policy.Bot{}, Challenge: &challenge.Challenge{RandomData: randomData}},
			wantErr: challenge.ErrInvalidInput,
		},
		{
			name:    "nil-challenge",
			values:  goodValues(),
			input:   &challenge.ValidateInput{Rule: bot},
			wantErr: challenge.ErrInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mkRequest(t, tt.values)

			err := i.Validate(req, lg, tt.input)

			if tt.errCheck != nil {
				if !tt.errCheck(err) {
					t.Fatalf("error check failed, got: %v", err)
				}
				return
			}

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got error %v, want %v", err, tt.wantErr)
			}
		})
	}
}
