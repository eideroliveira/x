package login

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// noticeFlashRoundTrip sets the notice flash on one response and reads it
// back from a request carrying the cookie that response set, so the value
// goes through net/http's own cookie sanitizer on the way.
func noticeFlashRoundTrip(t *testing.T, ne *NoticeError) *NoticeError {
	t.Helper()
	set := httptest.NewRecorder()
	SetNoticeFlash(set, ne)
	cookies := set.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("SetNoticeFlash set %d cookies, want 1", len(cookies))
	}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(cookies[0])
	return (&ViewHelper{}).GetNoticeFlash(httptest.NewRecorder(), r)
}

func TestNoticeFlashKeepsNonASCII(t *testing.T) {
	for _, msg := range []string{
		"Conta já existe. Por favor faça login.",
		"ã ç º",
		"level#and~separators; \"quotes\", commas",
		"",
	} {
		got := noticeFlashRoundTrip(t, &NoticeError{Level: NoticeLevel_Warn, Message: msg})
		if got == nil {
			t.Fatalf("%q: no notice read back", msg)
		}
		if got.Level != NoticeLevel_Warn || got.Message != msg {
			t.Errorf("read back %d %q, want %d %q", got.Level, got.Message, NoticeLevel_Warn, msg)
		}
	}
}

func TestNoticeFlashReadsLegacyRawCookie(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: noticeFlashCookieName, Value: "2#Conta j existe~ok"})
	got := (&ViewHelper{}).GetNoticeFlash(httptest.NewRecorder(), r)
	if got == nil || got.Level != NoticeLevel_Error || got.Message != "Conta j existe~ok" {
		t.Fatalf("legacy cookie read as %+v", got)
	}
}

func TestNoticeFlashRefusesMalformedCookie(t *testing.T) {
	for _, v := range []string{"", "no separator", "x~Zm9v", "1~not base64!"} {
		if got := decodeNoticeFlash(v); got != nil {
			t.Errorf("decodeNoticeFlash(%q) = %+v, want nil", v, got)
		}
	}
}
