// Package weberror contains within.website/x/web#Error all by itself.
package weberror

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
)

// NewError creates an Error based on an expected HTTP status code vs data populated
// from an HTTP response.
//
// This consumes the body of the HTTP response.
func New(wantStatusCode int, resp *http.Response) error {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	loc := resp.Request.URL

	return &Impl{
		WantStatus:   wantStatusCode,
		GotStatus:    resp.StatusCode,
		URL:          loc,
		Method:       resp.Request.Method,
		ResponseBody: string(data),
	}
}

// Impl is a web response error. Use this when API calls don't work out like you wanted them to.
type Impl struct {
	WantStatus, GotStatus int
	URL                   *url.URL
	Method                string
	ResponseBody          string
}

func (i Impl) Error() string {
	return fmt.Sprintf("%s %s: wanted status code %d, got: %d: %v", i.Method, i.URL, i.WantStatus, i.GotStatus, i.ResponseBody)
}

// LogValue formats this Error for slog.
func (i Impl) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Int("want_status", i.WantStatus),
		slog.Int("got_status", i.GotStatus),
		slog.String("url", i.URL.String()),
		slog.String("method", i.Method),
		slog.String("body", i.ResponseBody),
	)
}
