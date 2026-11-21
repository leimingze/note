package response

import (
	"encoding/json"
	"net/http"
)

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func JSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func Error400(w http.ResponseWriter, code, message string) {
	JSON(w, http.StatusBadRequest, Error{Code: code, Message: message})
}

func Error401(w http.ResponseWriter) {
	JSON(w, http.StatusUnauthorized, Error{Code: "UNAUTHORIZED", Message: "Invalid credentials"})
}

func Error404(w http.ResponseWriter, message string) {
	JSON(w, http.StatusNotFound, Error{Code: "NOT_FOUND", Message: message})
}

func Error409(w http.ResponseWriter, code, message string) {
	JSON(w, http.StatusConflict, Error{Code: code, Message: message})
}

func Error500(w http.ResponseWriter, message string) {
	JSON(w, http.StatusInternalServerError, Error{Code: "INTERNAL_ERROR", Message: message})
}
