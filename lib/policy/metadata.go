package policy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
)

type RequestMetadata struct {
	Context    context.Context
	RemoteAddr net.IP
	Method     string
	Path       string
	Query      url.Values
	Header     http.Header
	Proto      string
}

func MetadataFromRequest(r *http.Request) (*RequestMetadata, error) {
	host := r.Header.Get("X-Real-Ip")
	if host == "" {
		return &RequestMetadata{}, fmt.Errorf("[misconfiguration] X-Real-Ip header is not set")
	}

	addr := net.ParseIP(host)
	if addr == nil {
		return &RequestMetadata{}, fmt.Errorf("[misconfiguration] %q is not an IP address", addr)
	}

	meta := RequestMetadata{
		Context:    r.Context(),
		RemoteAddr: addr,
		Method:     r.Method,
		Path:       r.URL.Path,
		Query:      r.URL.Query(),
		Header:     r.Header,
	}

	return &meta, nil
}
