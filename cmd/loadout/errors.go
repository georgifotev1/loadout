package main

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"

	"github.com/gfotev/loadout/internal/auth"
	"github.com/gfotev/loadout/internal/catalog"
	"github.com/gfotev/loadout/internal/configurator"
	"github.com/gfotev/loadout/internal/platform/validate"
	"github.com/gfotev/loadout/internal/specs"
)

const statusClientClosedRequest = 499

// handleError is the one place errors become HTTP responses. Every error has the shape of
// Error in api/openapi.yaml: the HTTP status is the broad category, the code is a stable string
// the web app translates, and the message is for developers and logs, not for end users.
func (app *application) handleError(w http.ResponseWriter, r *http.Request, err error) {
	if verr := paramError(err); verr != nil {
		err = verr
	}
	var bodyErr *bodyError
	var validationErr *validate.Error
	var lockedErr *auth.LockedError
	switch {
	case errors.As(err, &bodyErr):
		app.writeError(w, r, http.StatusBadRequest, ErrorCodeInvalidBody, bodyErr.Error())
	case errors.As(err, &validationErr):
		app.writeValidation(w, r, validationErr)
	case errors.Is(err, catalog.ErrNotFound), errors.Is(err, specs.ErrNotFound), errors.Is(err, configurator.ErrNotFound):
		app.writeError(w, r, http.StatusNotFound, ErrorCodeNotFound, "resource not found")
	case errors.Is(err, auth.ErrUnauthenticated):
		app.writeError(w, r, http.StatusUnauthorized, ErrorCodeUnauthenticated, "not logged in")
	case errors.Is(err, auth.ErrInvalidCredentials):
		app.writeError(w, r, http.StatusUnauthorized, ErrorCodeInvalidCredentials, "wrong email or password")
	case errors.As(err, &lockedErr):
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(lockedErr.RetryAfter.Seconds()))))
		app.writeError(w, r, http.StatusTooManyRequests, ErrorCodeTooManyAttempts, "too many failed logins")
	case errors.Is(err, context.Canceled) && r.Context().Err() != nil:
		w.WriteHeader(statusClientClosedRequest)
	default:
		app.serverError(w, r, err)
	}
}

// paramError turns a parameter the generated router couldn't bind (page=abc) into a validation
// error, so the client sees the same 422 as for any other invalid input. It returns nil for
// any other error.
func paramError(err error) *validate.Error {
	var v validate.Validator
	var formatErr *InvalidParamFormatError
	var requiredErr *RequiredParamError
	var tooManyErr *TooManyValuesForParamError
	switch {
	case errors.As(err, &formatErr):
		v.Add(formatErr.ParamName, "invalid_format")
	case errors.As(err, &requiredErr):
		v.Add(requiredErr.ParamName, "required")
	case errors.As(err, &tooManyErr):
		v.Add(tooManyErr.ParamName, "too_many_values")
	default:
		return nil
	}
	var verr *validate.Error
	errors.As(v.Err(), &verr)
	return verr
}

// serverError logs err and writes a 500 that says nothing about the cause.
func (app *application) serverError(w http.ResponseWriter, r *http.Request, err error) {
	app.logger.Error("internal error", "err", err, "request_id", requestIDFrom(r.Context()), "method", r.Method, "path", r.URL.Path)
	app.writeError(w, r, http.StatusInternalServerError, ErrorCodeInternal, "internal server error")
}

func (app *application) writeValidation(w http.ResponseWriter, r *http.Request, verr *validate.Error) {
	fields := make([]FieldError, len(verr.Fields))
	for i, f := range verr.Fields {
		fields[i] = FieldError{Field: f.Field, Code: f.Code}
	}
	app.writeJSON(w, r, http.StatusUnprocessableEntity, Error{Error: ErrorDetail{
		Code:    ErrorCodeValidationFailed,
		Message: verr.Error(),
		Fields:  fields,
	}})
}

func (app *application) notFound(w http.ResponseWriter, r *http.Request) {
	app.writeError(w, r, http.StatusNotFound, ErrorCodeNotFound, "no such endpoint")
}

// writeError writes {"error": {"code", "message"}}. msg is for developers, not end users.
func (app *application) writeError(w http.ResponseWriter, r *http.Request, status int, code ErrorCode, msg string) {
	app.writeJSON(w, r, status, Error{Error: ErrorDetail{Code: code, Message: msg}})
}
