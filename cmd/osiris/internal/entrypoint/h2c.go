package entrypoint

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"

	"golang.org/x/net/http2"
)

func newH2CReverseProxy(target *url.URL) *httputil.ReverseProxy {
	target.Scheme = "http"

	director := func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
	}

	// Use h2c transport
	transport := &http2.Transport{
		AllowHTTP: true,
		DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
			// Just do plain TCP (h2c)
			return net.Dial(network, addr)
		},
	}

	return &httputil.ReverseProxy{
		Director:  director,
		Transport: transport,
	}
}
