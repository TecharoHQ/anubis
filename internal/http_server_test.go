package internal

import (
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestHTTPServerTimeouts(t *testing.T) {
	s := NewHTTPServer(http.NotFoundHandler())
	if s.ReadHeaderTimeout <= 0 || s.IdleTimeout <= 0 {
		t.Fatal("unbounded request connection")
	}
	if s.WriteTimeout != 0 {
		t.Fatal("streaming responses must remain supported")
	}
}

func TestHTTPServerClosesIncompleteHeaders(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	called := make(chan struct{}, 1)
	s := NewHTTPServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called <- struct{}{} }))
	s.ReadHeaderTimeout = 50 * time.Millisecond
	go s.Serve(listener)
	t.Cleanup(func() { s.Close() })
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(time.Second))
	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: example.com\r\nX-Incomplete: "); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Read(make([]byte, 1)); err != io.EOF {
		t.Fatalf("expected connection close, got %v", err)
	}
	select {
	case <-called:
		t.Fatal("incomplete request reached handler")
	default:
	}
}
