package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
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

	w.WriteHeader(statusCode)

	if _, err := w.Write(bytes); err != nil {
		l.ErrorContext(ctx, "failed to write response", slog.Any("error", err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
