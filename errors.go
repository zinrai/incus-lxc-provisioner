package main

import (
	"encoding/json"
	"net/http"
)

// apiError is the provisioner's uniform error body. It doubles as a Go error so the
// incus client can return it directly.
type apiError struct {
	Status  int    `json:"-"`
	Err     string `json:"error"`
	Message string `json:"message"`
}

func (e *apiError) Error() string { return e.Message }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func writeError(w http.ResponseWriter, status int, errID, msg string) {
	writeJSON(w, status, &apiError{Err: errID, Message: msg})
}

// writeIncusError turns an error from the Incus wrapper into an HTTP response.
// An *apiError carries the mapped status. Anything else is a transport failure.
func writeIncusError(w http.ResponseWriter, err error) {
	if e, ok := err.(*apiError); ok {
		writeJSON(w, e.Status, e)
		return
	}
	writeError(w, http.StatusServiceUnavailable, "incus_unreachable", err.Error())
}
