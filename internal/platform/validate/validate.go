// Package validate collects field-level input errors. Services and handlers return them as an
// *Error, and the API renders that as a 422 with one entry per field.
package validate

import "strings"

// FieldError says what is wrong with one field. Code is a stable string the web app translates
// (e.g. "required", "too_long", "invalid_email"), so it must not change once clients use it.
type FieldError struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

// Error is returned when input fails validation.
type Error struct {
	Fields []FieldError
}

func (e *Error) Error() string {
	parts := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		parts[i] = f.Field + ": " + f.Code
	}
	return "validation failed: " + strings.Join(parts, ", ")
}

// Validator accumulates field errors, keeping only the first one per field so the user sees one
// message per input.
//
//	var v validate.Validator
//	v.Check(req.Email != "", "email", "required")
//	v.Check(len(req.Name) <= 100, "name", "too_long")
//	if err := v.Err(); err != nil { ... }
type Validator struct {
	fields []FieldError
}

// Check records code for field when ok is false.
func (v *Validator) Check(ok bool, field, code string) {
	if !ok {
		v.Add(field, code)
	}
}

// Add records code for field unless the field already has an error.
func (v *Validator) Add(field, code string) {
	for _, f := range v.fields {
		if f.Field == field {
			return
		}
	}
	v.fields = append(v.fields, FieldError{Field: field, Code: code})
}

// Err returns nil if nothing was recorded, otherwise an *Error. It returns a plain nil error
// (not a nil *Error), so `if err != nil` works as expected.
func (v *Validator) Err() error {
	if len(v.fields) == 0 {
		return nil
	}
	return &Error{Fields: v.fields}
}

// Email reports whether s (already trimmed) looks like an email address. It's only a shape
// check: whether the address works shows when we send to it.
func Email(s string) bool {
	return len(s) <= 254 && strings.Count(s, "@") == 1 &&
		!strings.HasPrefix(s, "@") && !strings.HasSuffix(s, "@")
}
