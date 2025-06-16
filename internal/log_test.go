package internal

import (
	"bytes"
	"log"
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
