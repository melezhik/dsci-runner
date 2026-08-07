package git

// git related types and functions

import (
	"net/http"
)

// statusWriter intercepts and records the written HTTP status
type StatusWriter struct {
	http.ResponseWriter
	Status int
}

func (w *StatusWriter) WriteHeader(statusCode int) {
	w.Status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}
