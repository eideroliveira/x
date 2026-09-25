package login

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"gorm.io/gorm"
)

// A user model that can issue login codes (UserLoginCoder) but has no way to
// deliver them (no UserLoginCodeSender) must not get a send route. The handler
// used to save a fresh code on the account first and only then assert the
// sender, in single-value form: every POST overwrote the code the user was
// waiting for, then panicked into a 500.

// codeOnlyUser issues codes but cannot send them. FindUser hands back the
// registered model itself, so the test can read what the handler did to it.
type codeOnlyUser struct {
	UserPass
	UserLoginCode
	generated int
}

func (u *codeOnlyUser) FindUser(*gorm.DB, interface{}, string) (interface{}, error) {
	return u, nil
}

func (u *codeOnlyUser) GenerateLoginCode(*gorm.DB, interface{}) (string, error) {
	u.generated++
	return "123456", nil
}

// codeSendingUser can also deliver the code it issues.
type codeSendingUser struct {
	codeOnlyUser
	sent []string
}

func (u *codeSendingUser) FindUser(*gorm.DB, interface{}, string) (interface{}, error) {
	return u, nil
}

func (u *codeSendingUser) SendLoginCode(_ *http.Request, identifier, _ string) error {
	u.sent = append(u.sent, identifier)
	return nil
}

func loginCodeBuilder(model interface{}) *Builder {
	return New().Secret("test-secret").DB(&gorm.DB{}).UserModel(model)
}

func sendLoginCodeRequest(b *Builder) *http.Request {
	form := url.Values{"account": {"someone@example.com"}}
	r := httptest.NewRequest(http.MethodPost, b.sendLoginCodeURL, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func TestSendLoginCodeRouteNeedsASender(t *testing.T) {
	t.Run("no sender: no route, the validate route stays", func(t *testing.T) {
		b := loginCodeBuilder(&codeOnlyUser{})
		mux := http.NewServeMux()
		b.MountAPI(mux)

		if _, pattern := mux.Handler(sendLoginCodeRequest(b)); pattern != "" {
			t.Errorf("send route mounted at %q for a user model that cannot send a code", pattern)
		}
		validate := httptest.NewRequest(http.MethodPost, b.validateLoginCodeURL, nil)
		if _, pattern := mux.Handler(validate); pattern != b.validateLoginCodeURL {
			t.Errorf("validate route pattern = %q, want %q: codes issued elsewhere are still typed in", pattern, b.validateLoginCodeURL)
		}
	})

	t.Run("sender: route mounted", func(t *testing.T) {
		b := loginCodeBuilder(&codeSendingUser{})
		mux := http.NewServeMux()
		b.MountAPI(mux)

		if _, pattern := mux.Handler(sendLoginCodeRequest(b)); pattern != b.sendLoginCodeURL {
			t.Errorf("send route pattern = %q, want %q", pattern, b.sendLoginCodeURL)
		}
	})
}

func TestSendLoginCodeWithoutSenderSavesNoCode(t *testing.T) {
	u := &codeOnlyUser{}
	b := loginCodeBuilder(u)

	w := httptest.NewRecorder()
	func() {
		defer func() {
			if p := recover(); p != nil {
				t.Fatalf("handler panicked: %v", p)
			}
		}()
		b.sendUserCodeLogin(w, sendLoginCodeRequest(b))
	}()

	if u.generated != 0 {
		t.Errorf("a code was generated %d time(s) with no way to send it: it replaces the one the user is waiting for", u.generated)
	}
	if w.Code == http.StatusOK || w.Code >= http.StatusInternalServerError {
		t.Errorf("status = %d, want a refusal", w.Code)
	}
}
