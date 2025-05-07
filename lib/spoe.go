package lib

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/TecharoHQ/anubis"
	"github.com/dropmorepackets/haproxy-go/pkg/encoding"
)

type SpoeOptions struct {
	Pub ed25519.PublicKey
}

func (sopt *SpoeOptions) SpoeHandler(_ context.Context, w *encoding.ActionWriter, m *encoding.Message) {
	lg := slog.With("handle request")

	messageName := "check-client-cookie"
	actualMessageName := string(m.NameBytes())

	if actualMessageName != messageName {
		lg.Warn("Unknown message %s", m.NameBytes())
		return
	}

	lg.Debug(fmt.Sprintf("%+v", m))

	k := encoding.AcquireKVEntry()
	defer encoding.ReleaseKVEntry(k)

	found := false
	for m.KV.Next(k) {
		if k.NameEquals("cookie") {
			found = true
			break
		}
	}

	if !found {
		lg.Error("Mandatory value cookie not found")
		return
	}

	value := string(k.ValueBytes())

	cookie, err := http.ParseSetCookie(fmt.Sprintf("%s=%s", anubis.CookieName, value))
	if err != nil {
		lg.Error(fmt.Sprintf("Failed to parse cookie: %v", err))
		return
	}
	slog.Debug(fmt.Sprintf("%+v", cookie))
	valid, _ := ValidateCookie(cookie, lg, sopt.Pub, "spop")

	if valid {
		lg.Debug("cookie is valid")
		w.SetBool(encoding.VarScopeRequest, "valid", true)
	} else {
		lg.Debug("cookie is NOT valid")
	}
}
