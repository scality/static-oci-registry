package http

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog"
)

// RespondWithJSON writes the given data as a JSON response with the specified status code.
func RespondWithJSON(w http.ResponseWriter, data any, statusCode int, l *zerolog.Logger) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		l.Error().Err(err).Msg("failed to encode response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
