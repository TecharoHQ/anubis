package expressions

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"
)

// JoinOperator is a type wrapper for and/or operators.
//
// This is a separate type so that validation can be done at the type level.
type JoinOperator string

// Possible values for JoinOperator
const (
	JoinAnd JoinOperator = "&&"
	JoinOr  JoinOperator = "||"
)

// Valid ensures that JoinOperator is semantically valid.
func (jo JoinOperator) Valid() error {
	switch jo {
	case JoinAnd, JoinOr:
		return nil
	default:
		return ErrWrongJoinOperator
	}
}

var (
	ErrWrongJoinOperator = errors.New("expressions: invalid join operator")
	ErrNoExpressions     = errors.New("expressions: cannot join zero expressions")
	ErrCantCompile       = errors.New("expressions: can't compile one expression")
)

// JoinClauses joins a list of compiled clauses into one big if statement.
//
// Imagine the following two clauses:
//
//	ball.color == "red"
//	ball.shape == "round"
//
// JoinClauses would emit one "joined" clause such as:
//
//	( ball.color == "red" ) && ( ball.shape == "round" )
func JoinClauses(env *cel.Env, operator JoinOperator, clauses ...*cel.Ast) (*cel.Ast, error) {
	if err := operator.Valid(); err != nil {
		return nil, fmt.Errorf("%w: wanted && or ||, got: %q", err, operator)
	}

	switch len(clauses) {
	case 0:
		return nil, ErrNoExpressions
	case 1:
		return clauses[0], nil
	}

	var exprs []string
	var errs []error

	for _, clause := range clauses {
		clauseStr, err := cel.AstToString(clause)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		exprs = append(exprs, "( "+clauseStr+" )")
	}

	if len(errs) != 0 {
		return nil, fmt.Errorf("errors while decompiling statements: %w", errors.Join(errs...))
	}

	statement := strings.Join(exprs, " "+string(operator)+" ")
	result, iss := env.Compile(statement)
	if iss != nil {
		return nil, iss.Err()
	}

	return result, nil
}

func Join(env *cel.Env, operator JoinOperator, clauses ...string) (*cel.Ast, error) {
	var statements []*cel.Ast
	var errs []error

	for _, clause := range clauses {
		stmt, iss := env.Compile(clause)
		if iss != nil && iss.Err() != nil {
			errs = append(errs, fmt.Errorf("%w: %q gave: %w", ErrCantCompile, clause, iss.Err()))
			continue
		}
		statements = append(statements, stmt)
	}

	if len(errs) != 0 {
		return nil, fmt.Errorf("errors while joining clauses: %w", errors.Join(errs...))
	}

	return JoinClauses(env, operator, statements...)
}
