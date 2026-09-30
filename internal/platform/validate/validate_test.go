package validate

import (
	"errors"
	"testing"
)

func TestValidatorNoErrors(t *testing.T) {
	var v Validator
	v.Check(true, "email", "required")
	if err := v.Err(); err != nil {
		t.Fatalf("Err() = %v, want nil", err)
	}
}

func TestValidatorKeepsFirstErrorPerField(t *testing.T) {
	var v Validator
	v.Check(false, "email", "required")
	v.Check(false, "email", "invalid_email")
	v.Add("name", "too_long")

	var verr *Error
	if !errors.As(v.Err(), &verr) {
		t.Fatalf("Err() = %v, want *Error", v.Err())
	}
	want := []FieldError{{"email", "required"}, {"name", "too_long"}}
	if len(verr.Fields) != len(want) {
		t.Fatalf("Fields = %v, want %v", verr.Fields, want)
	}
	for i := range want {
		if verr.Fields[i] != want[i] {
			t.Errorf("Fields[%d] = %v, want %v", i, verr.Fields[i], want[i])
		}
	}
	if got := verr.Error(); got != "validation failed: email: required, name: too_long" {
		t.Errorf("Error() = %q", got)
	}
}
