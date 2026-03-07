package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
)

func responseWrapper(StatusCode int, Response []byte, responseWriter http.ResponseWriter) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(StatusCode)
	responseWriter.Write(Response)
}

func jsonError(w http.ResponseWriter, msg string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func isValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}

func isValidPassword(password string) bool {
	return len(password) >= 6
}

func isValidName(name string) bool {
	trimmed := strings.TrimSpace(name)
	return len(trimmed) > 0 && len(trimmed) <= 100
}
