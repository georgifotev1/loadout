package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/gfotev/loadout/internal/platform/validate"
)

func TestHandleError(t *testing.T) {
	var v validate.Validator
	v.Add("email", "invalid_email")
	validationErr := v.Err()

	bodyErr := readJSON(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{")), &struct{}{})

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   ErrorCode
		wantFields []FieldError
	}{
		{"body error", bodyErr, http.StatusBadRequest, "invalid_body", nil},
		{"validation", validationErr, http.StatusUnprocessableEntity, "validation_failed",
			[]FieldError{{Field: "email", Code: "invalid_email"}}},
		{"wrapped validation", fmt.Errorf("signup: %w", validationErr), http.StatusUnprocessableEntity, "validation_failed",
			[]FieldError{{Field: "email", Code: "invalid_email"}}},
		{"unbindable parameter", &InvalidParamFormatError{ParamName: "page", Err: errors.New("bad")},
			http.StatusUnprocessableEntity, "validation_failed",
			[]FieldError{{Field: "page", Code: "invalid_format"}}},
		{"unknown", errors.New("pq: connection refused"), http.StatusInternalServerError, "internal", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			newTestApp().handleError(rec, httptest.NewRequest(http.MethodGet, "/api/x", nil), tt.err)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			var body Error
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body %q: %v", rec.Body.String(), err)
			}
			if body.Error.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", body.Error.Code, tt.wantCode)
			}
			if !slices.Equal(body.Error.Fields, tt.wantFields) {
				t.Errorf("fields = %v, want %v", body.Error.Fields, tt.wantFields)
			}
		})
	}
}

// A 500 must not leak the underlying error to the client.
func TestHandleErrorHidesInternalDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestApp().handleError(rec, httptest.NewRequest(http.MethodGet, "/api/x", nil), errors.New("secret dsn"))
	if strings.Contains(rec.Body.String(), "secret") {
		t.Errorf("body leaks the error: %s", rec.Body.String())
	}
}

func TestHandleErrorClientGone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/x", nil).WithContext(ctx)

	newTestApp().handleError(rec, req, fmt.Errorf("query: %w", context.Canceled))
	if rec.Code != statusClientClosedRequest {
		t.Errorf("status = %d, want %d", rec.Code, statusClientClosedRequest)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", rec.Body.String())
	}
}
