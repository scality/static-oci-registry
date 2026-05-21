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
	statusCode int,
	l *slog.Logger,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		l.ErrorContext(ctx, "failed to encode response", slog.Any("error", err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
