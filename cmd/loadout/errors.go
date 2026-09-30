package main

import (
	"context"
	"errors"
	"net/http"

	"github.com/gfotev/loadout/internal/platform/validate"
)

const statusClientClosedRequest = 499

// errorBody is the JSON shape of every API error. The HTTP status is the broad category, Code
// is a stable string the web app translates (e.g. "not_found", "email_taken"), and Message is
// for developers and logs, not for end users. Fields is only set for "validation_failed".
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string                `json:"code"`
	Message string                `json:"message,omitempty"`
	Fields  []validate.FieldError `json:"fields,omitempty"`
}

func (app *application) handleError(w http.ResponseWriter, r *http.Request, err error) {
	var bodyErr *bodyError
	var validationErr *validate.Error
	switch {
	case errors.As(err, &bodyErr):
		app.writeError(w, r, http.StatusBadRequest, "invalid_body", bodyErr.Error())
	case errors.As(err, &validationErr):
		app.writeValidation(w, r, validationErr)
	case errors.Is(err, context.Canceled) && r.Context().Err() != nil:
		w.WriteHeader(statusClientClosedRequest)
	default:
		app.serverError(w, r, err)
	}
}

// serverError logs err and writes a 500 that says nothing about the cause.
func (app *application) serverError(w http.ResponseWriter, r *http.Request, err error) {
	app.logger.Error("internal error", "err", err, "request_id", requestIDFrom(r.Context()), "method", r.Method, "path", r.URL.Path)
	app.writeError(w, r, http.StatusInternalServerError, "internal", "internal server error")
}

func (app *application) writeValidation(w http.ResponseWriter, r *http.Request, verr *validate.Error) {
	app.writeJSON(w, r, http.StatusUnprocessableEntity, errorBody{Error: errorDetail{
		Code:    "validation_failed",
		Message: verr.Error(),
		Fields:  verr.Fields,
	}})
}

func (app *application) notFound(w http.ResponseWriter, r *http.Request) {
	app.writeError(w, r, http.StatusNotFound, "not_found", "no such endpoint")
}

// writeError writes {"error": {"code", "message"}}. msg is for developers, not end users.
func (app *application) writeError(w http.ResponseWriter, r *http.Request, status int, code, msg string) {
	app.writeJSON(w, r, status, errorBody{Error: errorDetail{Code: code, Message: msg}})
}
