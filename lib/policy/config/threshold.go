package config

import (
	"errors"
	"fmt"
)

var (
	ErrNoThresholdRulesDefined             = errors.New("config: no thresholds defined")
	ErrThresholdMustHaveName               = errors.New("config.Thresold: must set name")
	ErrThresholdMustHaveExpression         = errors.New("config.Threshold: must set expression")
	ErrThresholdChallengeMustHaveChallenge = errors.New("config.Threshold: a threshold with the CHALLENGE action must have challenge set")
)

type Threshold struct {
	Name       string            `json:"name" yaml:"name"`
	Expression *ExpressionOrList `json:"expression" yaml:"expression"`
	Action     Rule              `json:"action" yaml:"action"`
	Challenge  *ChallengeRules   `json:"challenge" yaml:"challenge"`
}

func (t Threshold) Valid() error {
	var errs []error

	if len(t.Name) == 0 {
		errs = append(errs, ErrThresholdMustHaveName)
	}

	if t.Expression == nil {
		errs = append(errs, ErrThresholdMustHaveExpression)
	}

	if t.Expression != nil {
		if err := t.Expression.Valid(); err != nil {
			errs = append(errs, err)
		}
	}

	if err := t.Action.Valid(); err != nil {
		errs = append(errs, err)
	}

	if t.Action == RuleChallenge && t.Challenge == nil {
		errs = append(errs, ErrThresholdChallengeMustHaveChallenge)
	}

	if t.Challenge != nil {
		if err := t.Challenge.Valid(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("config: threshold entry for %q is not valid:\n%w", t.Name, errors.Join(errs...))
	}

	return nil
}
