// Package maybedoer contains a pipeline of actions that might fail. If any action
// in the chain fails, no further actions take place and the error becomes the pipeline
// error.
package maybedoer

import (
	"fmt"
	"reflect"
	"runtime"
)

// Chain sequences a set of actions to be performed via calls to
// `Maybe` such that any previous error prevents new actions from being
// performed.
//
// This is, conceptually, just a go-ification of the Maybe monad.
type Chain struct {
	err error
}

// getFunctionName resolves the name of a function for debugging
// maybedoer Chains.
func getFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

// Maybe performs `f` if no previous call to a Maybe'd action resulted
// in an error
func (c *Chain) Maybe(f func() error) {
	if c.err == nil {
		if err := f(); err != nil {
			c.err = fmt.Errorf("%s: %w", getFunctionName(f), err)
		}
	}
}

// Error returns the first error encountered in the Error chain.
func (c *Chain) Error() error {
	return c.err
}
