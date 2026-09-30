package login

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"gorm.io/gorm"
)

// A user model's FindUser answers ErrAccountAmbiguous when the typed login
// matches more than one account. Every path that looks a user up by the typed
// login must refuse with FailCodeAccountAmbiguous — until this existed the
// password and code logins and the forgot-password form panicked on any
// FindUser error other than "not found", and the send-code form called it a
// system error.

type ambiguousUser struct {
	codeSendingUser
}

func (u *ambiguousUser) FindUser(*gorm.DB, interface{}, string) (interface{}, error) {
	return nil, ErrAccountAmbiguous
}

func TestEveryLoginPathRefusesAnAmbiguousAccount(t *testing.T) {
	b := New().Secret("test-secret").DB(&gorm.DB{}).UserModel(&ambiguousUser{})
	mux := http.NewServeMux()
	b.MountAPI(mux)

	for _, tc := range []struct {
		name string
		url  string
		form url.Values
	}{
		{"password login", b.passwordLoginURL, url.Values{"account": {"Pastor@Example.com"}, "password": {"secret"}}},
		{"login code", b.validateLoginCodeURL, url.Values{"account": {"Pastor@Example.com"}, "logincode": {"123456"}}},
		{"send login code", b.sendLoginCodeURL, url.Values{"account": {"Pastor@Example.com"}}},
		{"forgot password", b.sendResetPasswordLinkURL, url.Values{"account": {"Pastor@Example.com"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, tc.url, strings.NewReader(tc.form.Encode()))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			func() {
				defer func() {
					if p := recover(); p != nil {
						t.Fatalf("panicked on an ambiguous account: %v", p)
					}
				}()
				mux.ServeHTTP(w, r)
			}()
			var got string
			for _, c := range w.Result().Cookies() {
				if c.Name == failCodeFlashCookieName {
					got = c.Value
				}
			}
			if want := fmt.Sprint(FailCodeAccountAmbiguous); got != want {
				t.Errorf("fail code = %q, want %q (FailCodeAccountAmbiguous)", got, want)
			}
			if loc := w.Result().Header.Get("Location"); strings.Contains(loc, b.resetPasswordLinkSentPageURL) && tc.url == b.sendResetPasswordLinkURL {
				t.Errorf("forgot password redirected to the link-sent page (%q): nothing was sent", loc)
			}
		})
	}
}
