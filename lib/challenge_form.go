package lib

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
)

func prepareChallengeForm(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	err := r.ParseForm()
	if err == nil {
		err = r.ParseMultipartForm(64 << 10)
	}
	// An empty body is not malformed. Some reverse proxies strip the body of
	// auth subrequests (for example nginx' auth_request) while keeping the
	// original Content-Type, and a chunked request whose body has not been
	// buffered yet is indistinguishable from an empty one. Parsing an empty
	// multipart body fails with an io.EOF-wrapped error instead of
	// http.ErrNotMultipart, so tolerate it as well. Bodies that are actually
	// malformed fail with a different error and are still rejected.
	if err == nil || errors.Is(err, http.ErrNotMultipart) || errors.Is(err, io.EOF) {
		return true
	}
	var limit *http.MaxBytesError
	status := http.StatusBadRequest
	if errors.As(err, &limit) {
		status = http.StatusRequestEntityTooLarge
	}
	http.Error(w, http.StatusText(status), status)
	return false
}

func cleanupChallengeForm(r *http.Request) {
	if r.MultipartForm != nil {
		if err := r.MultipartForm.RemoveAll(); err != nil {
			slog.DebugContext(r.Context(), "can't remove multipart form files", "err", err)
		}
	}
}
