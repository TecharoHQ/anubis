package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestThresholdValid(t *testing.T) {
	for _, tt := range []struct {
		err   error
		input *Threshold
		name  string
	}{
		{
			name: "basic allow",
			input: &Threshold{
				Name:       "basic-allow",
				Expression: &ExpressionOrList{Expression: "true"},
				Action:     RuleAllow,
			},
			err: nil,
		},
		{
			name: "basic challenge",
			input: &Threshold{
				Name:       "basic-challenge",
				Expression: &ExpressionOrList{Expression: "true"},
				Action:     RuleChallenge,
				Challenge: &ChallengeRules{
					Algorithm:  "fast",
					Difficulty: 1,
					ReportAs:   1,
				},
			},
			err: nil,
		},
		{
			name:  "no name",
			input: &Threshold{},
			err:   ErrThresholdMustHaveName,
		},
		{
			name:  "no expression",
			input: &Threshold{},
			err:   ErrThresholdMustHaveName,
		},
		{
			name: "invalid expression",
			input: &Threshold{
				Expression: &ExpressionOrList{},
			},
			err: ErrExpressionEmpty,
		},
		{
			name:  "invalid action",
			input: &Threshold{},
			err:   ErrUnknownAction,
		},
		{
			name: "challenge action but no challenge",
			input: &Threshold{
				Action: RuleChallenge,
			},
			err: ErrThresholdChallengeMustHaveChallenge,
		},
		{
			name: "challenge invalid",
			input: &Threshold{
				Action:    RuleChallenge,
				Challenge: &ChallengeRules{Difficulty: -1, ReportAs: -1},
			},
			err: ErrChallengeDifficultyTooLow,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.input.Valid(); !errors.Is(err, tt.err) {
				t.Errorf("threshold is invalid: %v", err)
			}
		})
	}
}

func TestDefaultThresholdsValid(t *testing.T) {
	for i, th := range DefaultThresholds {
		t.Run(fmt.Sprintf("%d %s", i, th.Name), func(t *testing.T) {
			if err := th.Valid(); err != nil {
				t.Errorf("threshold invalid: %v", err)
			}
		})
	}
}

func TestLoadActuallyLoadsThresholds(t *testing.T) {
	fin, err := os.Open(filepath.Join(".", "testdata", "good", "thresholds.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	defer fin.Close()

	c, err := Load(fin, fin.Name())
	if err != nil {
		t.Fatal(err)
	}

	if len(c.Thresholds) != 4 {
		t.Errorf("wanted 4 thresholds, got %d thresholds", len(c.Thresholds))
	}
}
