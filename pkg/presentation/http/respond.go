package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
)

// RespondWithJSON writes the given data as a JSON response with the specified status code.
//
//nolint:revive // argument-limit: ctx is required for *Context log methods.
func RespondWithJSON(
	ctx context.Context,
	w http.ResponseWriter,
	data any,
	headers map[string]string,
	statusCode int,
	l *slog.Logger,
) {
	for key, value := range headers {
		w.Header().Set(key, value)
	}

	if _, ok := headers["Content-Type"]; !ok {
		w.Header().Set("Content-Type", "application/json")
	}

	// Content-Length is set automatically by net/http when Encode writes
	// the body in one shot without chunked encoding, matching what real
	// registries (Quay, MCR, GHCR) advertise on manifest responses.
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		l.ErrorContext(ctx, "failed to encode response", slog.Any("error", err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// RespondWithBytes writes the given bytes as a response with the specified status code.
//
//nolint:revive // argument-limit: ctx is required for *Context log methods.
func RespondWithBytes(
	ctx context.Context,
	w http.ResponseWriter,
	bytes []byte,
	headers map[string]string,
	statusCode int,
	l *slog.Logger,
) {
	for key, value := range headers {
		w.Header().Set(key, value)
	}

	if _, ok := headers["Content-Type"]; !ok {
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	// Content-Length is set automatically by net/http when Write delivers
	// the whole body in one call without chunked encoding, matching what
	// real registries (Quay, MCR, GHCR) advertise on manifest responses.
	w.WriteHeader(statusCode)

	if _, err := w.Write(bytes); err != nil {
		l.ErrorContext(ctx, "failed to write response", slog.Any("error", err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// RespondNoBody writes a response with the given headers and status code but
// no body. Per RFC 9110 §9.3.2 a HEAD response must carry the same headers as
// the GET equivalent (including Content-Length) but omit the payload, so
// callers pass the length the GET body would have had. Unlike the GET
// helpers, Content-Length must be set explicitly here: net/http only fills
// it in when something is actually written, and HEAD writes nothing.
func RespondNoBody(
	w http.ResponseWriter,
	headers map[string]string,
	statusCode int,
	bodyLen int,
) {
	for key, value := range headers {
		w.Header().Set(key, value)
	}

	if _, ok := headers["Content-Type"]; !ok {
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	w.Header().Set("Content-Length", strconv.Itoa(bodyLen))
	w.WriteHeader(statusCode)
}
