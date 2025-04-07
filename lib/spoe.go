package lib

import (
	"crypto/ed25519"
	"fmt"
	"github.com/TecharoHQ/anubis"
	"github.com/negasus/haproxy-spoe-go/action"
	"github.com/negasus/haproxy-spoe-go/request"
	"log/slog"
	"net/http"
)

type SpoeOptions struct {
	Pub ed25519.PublicKey
}

func (sopt *SpoeOptions) SpoeHandler(req *request.Request) {
	lg := slog.With("handle request EngineID", req.EngineID, "StreamID", req.StreamID, "FrameID", req.FrameID, "messages", req.Messages.Len())

	messageName := "check-client-cookie"

	mes, err := req.Messages.GetByName(messageName)
	if err != nil {
		lg.Info(fmt.Sprintf("message %s not found: %v", messageName, err))
		return
	}
	slog.Debug(fmt.Sprintf("%+v", mes))

	value, ok := mes.KV.Get("cookie")
	if !ok {
		lg.Info("var 'cookie' not found in message")
		return
	}
	slog.Debug(fmt.Sprintf("%v", value))

	if value != nil {
		cookie, err := http.ParseSetCookie(fmt.Sprintf("%s=%s", anubis.CookieName, value))
		if err != nil {
			lg.Error(fmt.Sprintf("Failed to parse cookie: %v", err))
			return
		}
		slog.Debug(fmt.Sprintf("%+v", cookie))
		valid, _ := ValidateCookie(cookie, lg, sopt.Pub, "spop")

		if valid {
			lg.Debug("cookie is valid")
			req.Actions.SetVar(action.ScopeRequest, "valid", true)
		} else {
			lg.Debug("cookie is NOT valid")
		}
	}
}
