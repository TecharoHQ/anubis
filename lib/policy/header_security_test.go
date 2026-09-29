package policy

import (
	"net/http/httptest"
	"testing"

	"github.com/TecharoHQ/anubis/lib/config"
)

func TestHeaderRulesReadAllValues(t *testing.T) {
	r := httptest.NewRequest("GET", "http://example.com/", nil)
	r.Header.Add("X-Test", "benign")
	r.Header.Add("X-Test", "evil")
	c, err := NewHeaderMatchesChecker("x-test", "evil")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := c.Check(r); err != nil || !got {
		t.Fatalf("regex match=%v err=%v", got, err)
	}
	cel, err := NewCELChecker(&config.ExpressionOrList{Expression: `"x-test" in headers && headers["x-test"] == "benign,evil"`}, newTestDNS(t), false)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := cel.Check(r); err != nil || !got {
		t.Fatalf("CEL match=%v err=%v", got, err)
	}
}

func TestAnchoredHeaderRulesReadEachValue(t *testing.T) {
	for _, values := range [][]string{{"benign", "evil"}, {"evil", "benign"}} {
		r := httptest.NewRequest("GET", "http://example.com/", nil)
		r.Header["X-Test"] = values
		c, err := NewHeaderMatchesChecker("X-Test", "^evil$")
		if err != nil {
			t.Fatal(err)
		}
		if got, err := c.Check(r); err != nil || !got {
			t.Errorf("values=%q match=%v err=%v", values, got, err)
		}
	}
	r := httptest.NewRequest("GET", "http://example.com/", nil)
	r.Header["X-Test"] = []string{"", ""}
	if got, _ := NewHeaderExistsChecker("X-Test").Check(r); got {
		t.Fatal("empty values treated as present")
	}
}
