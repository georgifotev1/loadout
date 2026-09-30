package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// maxBodyBytes is the request body limit for readJSON.
const maxBodyBytes = 1 << 20 // 1 MiB

// writeJSON writes v as JSON with the given status. Encoding failures are logged instead of
// making every handler do it.
func (app *application) writeJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		app.logger.Error("write json", "err", err, "path", r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(append(b, '\n')); err != nil {
		app.logger.Error("write json", "err", err, "path", r.URL.Path)
	}
}

// bodyError means the request body could not be decoded. Its message is safe to show to the
// client.
type bodyError struct {
	msg string
}

func (e *bodyError) Error() string {
	return e.msg
}

func bodyErrorf(format string, args ...any) error {
	return &bodyError{msg: fmt.Sprintf(format, args...)}
}

func readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		var maxErr *http.MaxBytesError
		switch {
		case errors.As(err, &syntaxErr):
			return bodyErrorf("body contains malformed JSON (at character %d)", syntaxErr.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return bodyErrorf("body contains malformed JSON")
		case errors.As(err, &typeErr):
			if typeErr.Field != "" {
				return bodyErrorf("body has the wrong type for field %q", typeErr.Field)
			}
			return bodyErrorf("body has the wrong type (at character %d)", typeErr.Offset)
		case errors.Is(err, io.EOF):
			return bodyErrorf("body must not be empty")
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			return bodyErrorf("body contains unknown field %s", strings.TrimPrefix(err.Error(), "json: unknown field "))
		case errors.As(err, &maxErr):
			return bodyErrorf("body must not be larger than %d bytes", maxErr.Limit)
		default:
			return err
		}
	}

	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return bodyErrorf("body must contain a single JSON value")
	}
	return nil
}
