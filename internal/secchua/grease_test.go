package secchua

import (
	"fmt"
	"testing"
)

func TestIsGREASE(t *testing.T) {
	for _, tt := range []struct {
		input string
		want  bool
	}{
		{"Chromium", false},
		// Variants I've seen in chrome, likely incomplete
		{"Not/A?Brand", true},
		{"Nota Brand", true},
		{"Not A;Brand", true},
		{" Not A;Brand", true},
		{"Not)A;Brand", true},
		{"Not?A_Brand", true},
	} {
		t.Run(fmt.Sprint(tt), func(t *testing.T) {
			if got := IsGREASE(tt.input); got != tt.want {
				t.Logf("want: %v", tt.want)
				t.Logf(" got: %v", got)
				t.Fatal("unexpected equality result")
			}
		})
	}
}
