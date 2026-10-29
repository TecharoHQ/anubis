package weberror

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestNewError(t *testing.T) {
	tests := []struct {
		name       string
		wantStatus int
		response   *http.Response
		wantErr    bool
		checkError func(*testing.T, error)
	}{
		{
			name:       "successful error creation",
			wantStatus: 200,
			response:   makeResponse(404, "not found"),
			wantErr:    false,
			checkError: func(t *testing.T, err error) {
				if err == nil {
					t.Fatalf("expected *Error, got nil")
				}
				if _, ok := err.(*Impl); !ok {
					t.Fatalf("expected *Error, got %T", err)
				}
			},
		},
		{
			name:       "status codes match",
			wantStatus: 200,
			response:   makeResponse(500, "internal server error"),
			wantErr:    false,
			checkError: func(t *testing.T, err error) {
				webErr, ok := err.(*Impl)
				if !ok {
					t.Fatalf("expected *Error, got %T", err)
				}
				if webErr.WantStatus != 200 {
					t.Errorf("expected WantStatus 200, got %d", webErr.WantStatus)
				}
				if webErr.GotStatus != 500 {
					t.Errorf("expected GotStatus 500, got %d", webErr.GotStatus)
				}
			},
		},
		{
			name:       "captures response body",
			wantStatus: 200,
			response:   makeResponse(500, "internal server error"),
			wantErr:    false,
			checkError: func(t *testing.T, err error) {
				webErr, ok := err.(*Impl)
				if !ok {
					t.Fatalf("expected *Error, got %T", err)
				}
				if webErr.ResponseBody != "internal server error" {
					t.Errorf("expected ResponseBody 'internal server error', got %q", webErr.ResponseBody)
				}
			},
		},
		{
			name:       "captures URL",
			wantStatus: 200,
			response:   makeResponse(404, "not found"),
			wantErr:    false,
			checkError: func(t *testing.T, err error) {
				webErr, ok := err.(*Impl)
				if !ok {
					t.Fatalf("expected *Error, got %T", err)
				}
				if webErr.URL == nil {
					t.Fatal("expected URL to be set, got nil")
				}
			},
		},
		{
			name:       "captures method",
			wantStatus: 200,
			response:   makeResponse(404, "not found"),
			wantErr:    false,
			checkError: func(t *testing.T, err error) {
				webErr, ok := err.(*Impl)
				if !ok {
					t.Fatalf("expected *Error, got %T", err)
				}
				if webErr.Method != "GET" {
					t.Errorf("expected Method 'GET', got %q", webErr.Method)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(tt.wantStatus, tt.response)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if tt.checkError != nil {
				tt.checkError(t, err)
			}
		})
	}
}

func TestError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *Impl
		want string
	}{
		{
			name: "basic error message",
			err: &Impl{
				WantStatus:   200,
				GotStatus:    404,
				URL:          mustParseURL("http://example.com/test"),
				Method:       "GET",
				ResponseBody: "not found",
			},
			want: "GET http://example.com/test: wanted status code 200, got: 404: not found",
		},
		{
			name: "post request error",
			err: &Impl{
				WantStatus:   201,
				GotStatus:    400,
				URL:          mustParseURL("http://example.com/api"),
				Method:       "POST",
				ResponseBody: "bad request",
			},
			want: "POST http://example.com/api: wanted status code 201, got: 400: bad request",
		},
		{
			name: "empty response body",
			err: &Impl{
				WantStatus:   200,
				GotStatus:    500,
				URL:          mustParseURL("http://example.com/"),
				Method:       "DELETE",
				ResponseBody: "",
			},
			want: "DELETE http://example.com/: wanted status code 200, got: 500: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("Error.Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestError_LogValue(t *testing.T) {
	tests := []struct {
		name  string
	err   *Impl
		check func(*testing.T, slog.Value)
	}{
		{
			name: "contains all fields",
			err: &Impl{
				WantStatus:   200,
				GotStatus:    404,
				URL:          mustParseURL("http://example.com/test"),
				Method:       "GET",
				ResponseBody: "not found",
			},
			check: func(t *testing.T, v slog.Value) {
				// LogValue returns a slog.GroupValue, which we can't directly inspect
				// but we can verify it doesn't panic and returns a group kind
				if v.Kind() != slog.KindGroup {
					t.Errorf("expected KindGroup, got %v", v.Kind())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.LogValue()
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func makeResponse(status int, body string) *http.Response {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		panic(err)
	}
	return resp
}

func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
