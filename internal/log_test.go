package internal

import (
	"bytes"
	"log"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

func TestErrorLogFilter(t *testing.T) {
	var buf bytes.Buffer
	destLogger := log.New(&buf, "", 0)
	errorFilterWriter := &ErrorLogFilter{Unwrap: destLogger}
	testErrorLogger := log.New(errorFilterWriter, "", 0)

	// Test Case 1: Suppressed message
	suppressedMessage := "http: proxy error: context canceled"
	testErrorLogger.Println(suppressedMessage)

	if buf.Len() != 0 {
		t.Errorf("Suppressed message was written to output. Output: %q", buf.String())
	}
	buf.Reset()

	// Test Case 2: Allowed message
	allowedMessage := "http: another error occurred"
	testErrorLogger.Println(allowedMessage)

	output := buf.String()
	if !strings.Contains(output, allowedMessage) {
		t.Errorf("Allowed message was not written to output. Output: %q", output)
	}
	if !strings.HasSuffix(output, "\n") {
		t.Errorf("Allowed message output is missing newline. Output: %q", output)
	}
	buf.Reset()

	// Test Case 3: Partially matching message (should be suppressed)
	partiallyMatchingMessage := "Some other log before http: proxy error: context canceled and after"
	testErrorLogger.Println(partiallyMatchingMessage)

	if buf.Len() != 0 {
		t.Errorf("Partially matching message was written to output. Output: %q", buf.String())
	}
	buf.Reset()
}

func TestGetRequestLogger(t *testing.T) {
	// Test case 1: Normal request with Host header
	req1, _ := http.NewRequest("GET", "http://example.com/test", nil)
	req1.Host = "example.com"

	logger := slog.Default()
	reqLogger := GetRequestLogger(logger, req1)

	// We can't easily test the actual log output without setting up a test handler,
	// but we can verify the function doesn't panic and returns a logger
	if reqLogger == nil {
		t.Error("GetRequestLogger returned nil")
	}

	// Test case 2: Subrequest auth mode with X-Forwarded-Host
	req2, _ := http.NewRequest("GET", "http://test.com/auth", nil)
	req2.Host = ""
	req2.Header.Set("X-Forwarded-Host", "original-site.com")

	reqLogger2 := GetRequestLogger(logger, req2)
	if reqLogger2 == nil {
		t.Error("GetRequestLogger returned nil for X-Forwarded-Host case")
	}

	// Test case 3: No host information available
	req3, _ := http.NewRequest("GET", "http://test.com/nohost", nil)
	req3.Host = ""

	reqLogger3 := GetRequestLogger(logger, req3)
	if reqLogger3 == nil {
		t.Error("GetRequestLogger returned nil for no host case")
	}
}
