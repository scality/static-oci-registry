package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// RespondWithJSON writes the given data as a JSON response with the specified status code.
func RespondWithJSON(w http.ResponseWriter, data any, statusCode int, l *slog.Logger) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		l.Error("failed to encode response", slog.Any("error_message", err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
