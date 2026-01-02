package internal

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	proxyproto "github.com/pires/go-proxyproto"
)

func TestWrapListenerWithProxyProtocol_Disabled(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	wrapped := WrapListenerWithProxyProtocol(ln, ProxyProtocolConfig{
		Enabled: false,
	})

	if wrapped != ln {
		t.Error("expected listener to be unchanged when disabled")
	}
}

func TestWrapListenerWithProxyProtocol_Enabled(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	wrapped := WrapListenerWithProxyProtocol(ln, ProxyProtocolConfig{
		Enabled: true,
		Timeout: 5 * time.Second,
	})

	if wrapped == ln {
		t.Error("expected listener to be wrapped when enabled")
	}

	_, ok := wrapped.(*proxyproto.Listener)
	if !ok {
		t.Error("expected wrapped listener to be *proxyproto.Listener")
	}
}

func TestWrapListenerWithProxyProtocol_DefaultTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	wrapped := WrapListenerWithProxyProtocol(ln, ProxyProtocolConfig{
		Enabled: true,
		Timeout: 0,
	})

	ppLn, ok := wrapped.(*proxyproto.Listener)
	if !ok {
		t.Fatal("expected wrapped listener to be *proxyproto.Listener")
	}

	if ppLn.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("expected default timeout of 5s, got %v", ppLn.ReadHeaderTimeout)
	}
}

func TestProxyProtoConnContext_Disabled(t *testing.T) {
	fn := ProxyProtoConnContext(false)
	if fn != nil {
		t.Error("expected nil when disabled")
	}
}

func TestProxyProtoConnContext_Enabled(t *testing.T) {
	fn := ProxyProtoConnContext(true)
	if fn == nil {
		t.Fatal("expected non-nil when enabled")
	}
}

func TestProxyProtocolXRealIP_Disabled(t *testing.T) {
	called := false
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	wrapped := ProxyProtocolXRealIP(false, h)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	wrapped.ServeHTTP(w, r)

	if !called {
		t.Error("handler was not called")
	}
}

func TestProxyProtocolXRealIP_NoConnection(t *testing.T) {
	var realIP string
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		realIP = r.Header.Get("X-Real-IP")
		w.WriteHeader(http.StatusOK)
	})

	wrapped := ProxyProtocolXRealIP(true, h)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	wrapped.ServeHTTP(w, r)

	if realIP != "" {
		t.Errorf("expected empty X-Real-IP without proxy proto connection, got %q", realIP)
	}
}

func TestProxyProtocolIntegration(t *testing.T) {
	var receivedIP string
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedIP = r.Header.Get("X-Real-IP")
		w.WriteHeader(http.StatusOK)
	})

	wrapped := ProxyProtocolXRealIP(true, h)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	ppLn := WrapListenerWithProxyProtocol(ln, ProxyProtocolConfig{
		Enabled: true,
		Timeout: 5 * time.Second,
	})

	srv := &http.Server{
		Handler:     wrapped,
		ConnContext: ProxyProtoConnContext(true),
	}

	go func() {
		srv.Serve(ppLn)
	}()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	header := &proxyproto.Header{
		Version:           1,
		Command:           proxyproto.PROXY,
		TransportProtocol: proxyproto.TCPv4,
		SourceAddr: &net.TCPAddr{
			IP:   net.ParseIP("203.0.113.42"),
			Port: 12345,
		},
		DestinationAddr: &net.TCPAddr{
			IP:   net.ParseIP("192.0.2.1"),
			Port: 80,
		},
	}

	_, err = header.WriteTo(conn)
	if err != nil {
		t.Fatal(err)
	}

	_, err = fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: test\r\n\r\n")
	if err != nil {
		t.Fatal(err)
	}

	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if receivedIP != "203.0.113.42" {
		t.Errorf("expected X-Real-IP to be 203.0.113.42, got %q", receivedIP)
	}
}

func TestProxyProtocolIntegrationV2(t *testing.T) {
	var receivedIP string
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedIP = r.Header.Get("X-Real-IP")
		w.WriteHeader(http.StatusOK)
	})

	wrapped := ProxyProtocolXRealIP(true, h)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	ppLn := WrapListenerWithProxyProtocol(ln, ProxyProtocolConfig{
		Enabled: true,
		Timeout: 5 * time.Second,
	})

	srv := &http.Server{
		Handler:     wrapped,
		ConnContext: ProxyProtoConnContext(true),
	}

	go func() {
		srv.Serve(ppLn)
	}()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	header := &proxyproto.Header{
		Version:           2,
		Command:           proxyproto.PROXY,
		TransportProtocol: proxyproto.TCPv4,
		SourceAddr: &net.TCPAddr{
			IP:   net.ParseIP("198.51.100.10"),
			Port: 54321,
		},
		DestinationAddr: &net.TCPAddr{
			IP:   net.ParseIP("192.0.2.1"),
			Port: 443,
		},
	}

	_, err = header.WriteTo(conn)
	if err != nil {
		t.Fatal(err)
	}

	_, err = fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: test\r\n\r\n")
	if err != nil {
		t.Fatal(err)
	}

	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if receivedIP != "198.51.100.10" {
		t.Errorf("expected X-Real-IP to be 198.51.100.10, got %q", receivedIP)
	}
}

func TestProxyProtocolIntegrationIPv6(t *testing.T) {
	var receivedIP string
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedIP = r.Header.Get("X-Real-IP")
		w.WriteHeader(http.StatusOK)
	})

	wrapped := ProxyProtocolXRealIP(true, h)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	ppLn := WrapListenerWithProxyProtocol(ln, ProxyProtocolConfig{
		Enabled: true,
		Timeout: 5 * time.Second,
	})

	srv := &http.Server{
		Handler:     wrapped,
		ConnContext: ProxyProtoConnContext(true),
	}

	go func() {
		srv.Serve(ppLn)
	}()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	header := &proxyproto.Header{
		Version:           2,
		Command:           proxyproto.PROXY,
		TransportProtocol: proxyproto.TCPv6,
		SourceAddr: &net.TCPAddr{
			IP:   net.ParseIP("2001:db8::1"),
			Port: 12345,
		},
		DestinationAddr: &net.TCPAddr{
			IP:   net.ParseIP("2001:db8::2"),
			Port: 80,
		},
	}

	_, err = header.WriteTo(conn)
	if err != nil {
		t.Fatal(err)
	}

	_, err = fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: test\r\n\r\n")
	if err != nil {
		t.Fatal(err)
	}

	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if receivedIP != "2001:db8::1" {
		t.Errorf("expected X-Real-IP to be 2001:db8::1, got %q", receivedIP)
	}
}
