package rest

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase/irepository"
)

// responder writes JSON responses and maps usecase errors to HTTP statuses.
type responder struct{}

func (responder) json(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (r responder) error(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, irepository.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, usecase.ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.Is(err, usecase.ErrNothingRunning):
		status = http.StatusConflict
	}
	if status == http.StatusInternalServerError {
		slog.Error("request failed", "err", err)
	}
	r.json(w, status, map[string]string{"error": err.Error()})
}

func (r responder) id(w http.ResponseWriter, req *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(req.PathValue("id"), 10, 64)
	if err != nil {
		r.json(w, http.StatusBadRequest, map[string]string{"error": "id must be a number"})
		return 0, false
	}
	return id, true
}

func (r responder) decode(w http.ResponseWriter, req *http.Request, v any) bool {
	if err := json.NewDecoder(req.Body).Decode(v); err != nil {
		r.json(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body: " + err.Error()})
		return false
	}
	return true
}
