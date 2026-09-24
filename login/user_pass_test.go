package login

import (
	"encoding/base64"
	"testing"
)

func TestResetPasswordTokenMatches(t *testing.T) {
	// The shape GenerateResetPasswordToken issues: base64 of a UUID string.
	stored := base64.URLEncoding.EncodeToString([]byte("0d5b29f8-7d1e-4b12-a4f3-9c6e1d5b7a20"))

	for _, c := range []struct {
		name      string
		submitted string
		stored    string
		want      bool
	}{
		{name: "same token", submitted: stored, stored: stored, want: true},
		{name: "wrong token of the same length", submitted: "X" + stored[1:], stored: stored, want: false},
		{name: "last byte differs", submitted: stored[:len(stored)-1] + "x", stored: stored, want: false},
		{name: "prefix of the token", submitted: stored[:len(stored)/2], stored: stored, want: false},
		{name: "token plus a suffix", submitted: stored + "A", stored: stored, want: false},
		{name: "empty submitted", submitted: "", stored: stored, want: false},
		{name: "empty stored, empty submitted", submitted: "", stored: "", want: false},
		{name: "empty stored, token submitted", submitted: stored, stored: "", want: false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := ResetPasswordTokenMatches(c.submitted, c.stored); got != c.want {
				t.Errorf("ResetPasswordTokenMatches(%q, %q) = %v, want %v", c.submitted, c.stored, got, c.want)
			}
		})
	}
}
