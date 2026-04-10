package lib

import (
	"bufio"
	"context"
	"fmt"
	"github.com/TecharoHQ/anubis/lib/policy/checker"
	"log/slog"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/policy"
	"github.com/dropmorepackets/haproxy-go/pkg/encoding"
)

var (
	// SPOEStatusPass indicates the client may pass without verification
	SPOEStatusPass uint32 = 0
	// SPOEStatusCookieOK indicates the client may pass, as it has previously completed a challenge
	SPOEStatusCookieOK uint32 = 1
	// SPOEStatusBlock indicates this client should not receive any service at all
	SPOEStatusBlock uint32 = 2
	// SPOEStatusChallenge indicates this request should be forwarded to anubis to initiate a validation flow
	SPOEStatusChallenge uint32 = 3

	// SPOEStatusError indicates an error occurred while trying to check the client. The anubis logs should give more
	// information on the details of the error
	SPOEStatusError uint32 = 255
)

type SpoeOptions struct {
	Server *Server
}

func (spoe *SpoeOptions) SpoeHandler(ctx context.Context, w *encoding.ActionWriter, m *encoding.Message) {
	lg := slog.With("handle request")

	messageName := "check-client-cookie"
	actualMessageName := string(m.NameBytes())

	if actualMessageName != messageName {
		lg.Warn(fmt.Sprintf("Unknown message %s", string(m.NameBytes())))
		return
	}

	lg.Debug(fmt.Sprintf("%+v", m))

	k := encoding.AcquireKVEntry()
	defer encoding.ReleaseKVEntry(k)

	var cookie string
	metadata := checker.RequestMetadata{
		Context: ctx,
	}

	for m.KV.Next(k) {
		name := string(k.NameBytes())
		switch name {
		case "cookie":
			cookie = string(k.ValueBytes())
			break
		case "path":
			value := string(k.ValueBytes())
			// we are (ab)using url to parse the path here, as it makes our life easier than doing it ourselves
			parsedUrl, err := url.Parse(value)
			if err != nil {
				lg.Error(fmt.Sprintf("path(%s) could not be parsed", value))
				w.SetUInt32(encoding.VarScopeRequest, "result", SPOEStatusError)
				return
			}
			metadata.Path = parsedUrl.Path
			metadata.Query = parsedUrl.Query()
			break
		case "headers":
			headers, err := parseHeaders(k.ValueBytes())
			if err != nil {
				lg.Error(fmt.Sprintf("headers could not be parsed: %s", err))
				w.SetUInt32(encoding.VarScopeRequest, "result", SPOEStatusError)
				return
			}
			metadata.Header = headers
			break
		case "method":
			metadata.Method = string(k.ValueBytes())
			break
		case "remote_addr":
			value := k.ValueBytes()
			if len(value) != 4 && len(value) != 16 {
				lg.Error(fmt.Sprintf("remote_addr (\"%s\") is no ip address", value))
				w.SetUInt32(encoding.VarScopeRequest, "result", SPOEStatusError)
				return
			}
			metadata.RemoteAddr = value
			break
		case "proto":
			metadata.Proto = string(k.ValueBytes())
		}
	}

	// Some codepaths (like the cookie restriction check) require this header
	metadata.Header.Set("X-Real-IP", metadata.RemoteAddr.String())

	result, rule, err := spoe.Server.check(&metadata, lg)
	if err != nil {
		lg.Error(fmt.Sprintf("Error checking ruleset: %v", err))
		w.SetUInt32(encoding.VarScopeRequest, "result", SPOEStatusError)
		return
	}

	valid, err := spoe.handleCookie(cookie, rule, metadata.Path, metadata.Header, lg)
	if err != nil {
		lg.Error(fmt.Sprintf("Error handling cookie: %v", err))
		// We do not return here, because even if there is a problem with the cookie, we can still proceed through other means
	}
	slog.Debug(fmt.Sprintf("%+v", cookie))

	if valid {
		lg.Debug("cookie is valid")
		w.SetUInt32(encoding.VarScopeRequest, "result", SPOEStatusCookieOK)
		return
	} else {
		lg.Debug("cookie is NOT valid")
	}

	err = spoe.interpretResult(result, rule, lg, w)
	if err != nil {
		lg.Error(fmt.Sprintf("Error performing ruleset action: %v", err))
		w.SetUInt32(encoding.VarScopeRequest, "result", SPOEStatusError)
	}
}

func (spoe *SpoeOptions) handleCookie(cookieValue string, rule *policy.Bot, path string, header http.Header, lg *slog.Logger) (bool, error) {
	cookie, err := http.ParseSetCookie(fmt.Sprintf("%s=%s", anubis.CookieName, cookieValue))
	if err != nil {
		return false, fmt.Errorf("parse cookie: %v", err)
	}
	slog.Debug(fmt.Sprintf("%+v", cookie))
	return ValidateCookie(cookie, lg, spoe.Server, rule, path, header), nil
}

func (spoe *SpoeOptions) interpretResult(cr policy.CheckResult, rule *policy.Bot, lg *slog.Logger, w *encoding.ActionWriter) error {
	switch cr.Rule {
	case config.RuleAllow:
		lg.Debug("VERDICT: allowing traffic to origin (explicit)")
		w.SetUInt32(encoding.VarScopeRequest, "result", SPOEStatusPass)
		return nil
	case config.RuleDeny:
		lg.Debug("VERDICT: explicit deny")
		if rule == nil {
			return fmt.Errorf("rule is nil, cannot calculate checksum")
		}
		hash := rule.Hash()

		lg.Debug("rule hash", "hash", hash)
		w.SetUInt32(encoding.VarScopeRequest, "result", SPOEStatusBlock)
		return nil
	case config.RuleChallenge:
		w.SetUInt32(encoding.VarScopeRequest, "result", SPOEStatusChallenge)
		lg.Debug("VERDICT: challenge requested")
		return nil
	case config.RuleBenchmark:
		// Benchmark is treated just like challenge, as it needs to be routed to anubis
		w.SetUInt32(encoding.VarScopeRequest, "result", SPOEStatusChallenge)
		lg.Debug("VERDICT: serving benchmark page (challenge)")
		return nil
	default:
		return fmt.Errorf("CONFIG ERROR: unknown rule %s", cr.Rule)
	}
}

func parseHeaders(headerData []byte) (ret map[string][]string, err error) {
	return textproto.NewReader(bufio.NewReader(strings.NewReader(string(headerData)))).ReadMIMEHeader()
}
