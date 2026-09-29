package lib

import (
	"errors"
	"log/slog"
	"net/http"
)

func prepareChallengeForm(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	err := r.ParseForm()
	if err == nil {
		err = r.ParseMultipartForm(64 << 10)
	}
	if err == nil || errors.Is(err, http.ErrNotMultipart) {
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
