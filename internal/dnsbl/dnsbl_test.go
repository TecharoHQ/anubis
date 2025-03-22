package dnsbl

import (
	"fmt"
	"net"
	"os"
	"testing"
)

func TestReverse4(t *testing.T) {
	cases := []struct {
		inp, out string
	}{
		{"1.2.3.4", "4.3.2.1"},
	}

	for _, cs := range cases {
		t.Run(fmt.Sprintf("%s->%s", cs.inp, cs.out), func(t *testing.T) {
			out := reverse4(net.ParseIP(cs.inp))

			if out != cs.out {
				t.Errorf("wanted %s\ngot:   %s", cs.out, out)
			}
		})
	}
}

func TestReverse6(t *testing.T) {
	cases := []struct {
		inp, out string
	}{
		{
			inp: "1234:5678:9ABC:DEF0:1234:5678:9ABC:DEF0",
			out: "0.f.e.d.c.b.a.9.8.7.6.5.4.3.2.1.0.f.e.d.c.b.a.9.8.7.6.5.4.3.2.1",
		},
	}

	for _, cs := range cases {
		t.Run(fmt.Sprintf("%s->%s", cs.inp, cs.out), func(t *testing.T) {
			out := reverse6(net.ParseIP(cs.inp))

			if out != cs.out {
				t.Errorf("wanted %s, got: %s", cs.out, out)
			}
		})
	}
}

func TestLookup(t *testing.T) {
	if os.Getenv("DONT_USE_NETWORK") != "" {
		t.Skip("test requires network egress")
		return
	}

	resp, err := Lookup("27.65.243.194")
	if err != nil {
		t.Fatalf("it broked: %v", err)
	}

	t.Logf("response: %d", resp)
}
