package policy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWeighDefaultSurvivesParsing(t *testing.T) {
	for _, tc := range []struct {
		name, weight string
		want         int
	}{
		{"default", "", 5}, {"explicit", "    weight:\n      adjust: 17\n", 17},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := "bots:\n  - name: weigh\n    user_agent_regex: .*\n    action: WEIGH\n" + tc.weight
			p, err := ParseConfig(t.Context(), strings.NewReader(input), "test.yaml", 4, "info", false)
			if err != nil {
				t.Fatal(err)
			}
			for _, b := range p.Bots {
				if b.Name == "weigh" {
					if b.Weight == nil {
						t.Fatal("missing default weight")
					}
					if b.Weight.Adjust != tc.want {
						t.Fatalf("weight=%d", b.Weight.Adjust)
					}
					return
				}
			}
			t.Fatal("missing rule")
		})
	}
}

func TestImportedWeighDefaultSurvivesParsing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bots.yaml")
	if err := os.WriteFile(path, []byte("- name: imported-weigh\n  user_agent_regex: .*\n  action: WEIGH\n"), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := ParseConfig(t.Context(), strings.NewReader("bots:\n  - import: "+path+"\n"), "test.yaml", 4, "info", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range p.Bots {
		if b.Name == "imported-weigh" {
			if b.Weight == nil || b.Weight.Adjust != 5 {
				t.Fatalf("weight=%v", b.Weight)
			}
			return
		}
	}
	t.Fatal("missing imported rule")
}
