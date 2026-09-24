package login

import (
	"testing"
	"time"
)

func TestLoginCodeMatches(t *testing.T) {
	const stored = "042917"

	for _, c := range []struct {
		name      string
		submitted string
		stored    string
		want      bool
	}{
		{name: "same code", submitted: stored, stored: stored, want: true},
		{name: "last digit differs", submitted: "042918", stored: stored, want: false},
		{name: "first digit differs", submitted: "142917", stored: stored, want: false},
		{name: "prefix of the code", submitted: stored[:3], stored: stored, want: false},
		{name: "code plus a suffix", submitted: stored + "0", stored: stored, want: false},
		{name: "empty submitted", submitted: "", stored: stored, want: false},
		{name: "empty stored, empty submitted", submitted: "", stored: "", want: false},
		{name: "empty stored, code submitted", submitted: stored, stored: "", want: false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := LoginCodeMatches(c.submitted, c.stored); got != c.want {
				t.Errorf("LoginCodeMatches(%q, %q) = %v, want %v", c.submitted, c.stored, got, c.want)
			}
		})
	}
}

func TestGetLoginCode(t *testing.T) {
	past := time.Now().Add(-time.Minute)
	future := time.Now().Add(10 * time.Minute)

	for _, c := range []struct {
		name        string
		code        UserLoginCode
		wantCode    string
		wantExpired bool
	}{
		{name: "never issued", code: UserLoginCode{}, wantCode: ""},
		{name: "code without an expiry", code: UserLoginCode{LoginCode: "042917"}, wantCode: ""},
		{name: "live code", code: UserLoginCode{LoginCode: "042917", LoginCodeExpiredAt: &future}, wantCode: "042917"},
		{name: "expired code", code: UserLoginCode{LoginCode: "042917", LoginCodeExpiredAt: &past}, wantExpired: true},
		{name: "used code", code: UserLoginCode{LoginCode: "", LoginCodeExpiredAt: &past}, wantExpired: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, _, expired := c.code.GetLoginCode()
			if got != c.wantCode || expired != c.wantExpired {
				t.Errorf("GetLoginCode() = (%q, expired %v), want (%q, expired %v)", got, expired, c.wantCode, c.wantExpired)
			}
		})
	}
}
