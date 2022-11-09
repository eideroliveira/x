package login

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/qor5/web"
	"github.com/qor5/x/i18n"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	h "github.com/theplant/htmlgo"
	"golang.org/x/text/language"
	"gorm.io/gorm"
)

var (
	ErrUserNotFound        = errors.New("user not found")
	ErrUserPassChanged     = errors.New("password changed")
	ErrWrongPassword       = errors.New("wrong password")
	ErrUserLocked          = errors.New("user locked")
	ErrUserGetLocked       = errors.New("user get locked")
	ErrWrongTOTPCode       = errors.New("wrong totp code")
	ErrTOTPCodeHasBeenUsed = errors.New("totp code has been used")
	ErrEmptyPassword       = errors.New("empty password")
	ErrPasswordNotMatch    = errors.New("password not match")
)

type HomeURLFunc func(r *http.Request, user interface{}) string
type NotifyUserOfResetPasswordLinkFunc func(user interface{}, resetLink string) error
type PasswordValidationFunc func(password string) (message string, ok bool)
type HookFunc func(r *http.Request, user interface{}, vals ...interface{}) error

type void struct{}

type Provider struct {
	Goth goth.Provider
	Key  string
	Text string
	Logo h.HTMLComponent
}

type CookieConfig struct {
	Path     string
	Domain   string
	SameSite http.SameSite
}

type RecaptchaConfig struct {
	SiteKey   string
	SecretKey string
}

type Builder struct {
	secret                string
	providers             []*Provider
	authCookieName        string
	authSecureCookieName  string
	continueUrlCookieName string
	// seconds
	sessionMaxAge        int
	cookieConfig         CookieConfig
	recaptchaEnabled     bool
	recaptchaConfig      RecaptchaConfig
	autoExtendSession    bool
	maxRetryCount        int
	noForgetPasswordLink bool

	// Common URLs
	homePageURLFunc HomeURLFunc
	loginPageURL    string
	LogoutURL       string
	allowURLs       map[string]void

	// TOTP URLs
	validateTOTPURL     string
	totpSetupPageURL    string
	totpValidatePageURL string

	// OAuth URLs
	oauthBeginURL            string
	oauthCallbackURL         string
	oauthCallbackCompleteURL string

	// UserPass URLs
	passwordLoginURL             string
	resetPasswordURL             string
	resetPasswordPageURL         string
	changePasswordURL            string
	changePasswordPageURL        string
	forgetPasswordPageURL        string
	sendResetPasswordLinkURL     string
	resetPasswordLinkSentPageURL string

	loginPageFunc                 web.PageFunc
	forgetPasswordPageFunc        web.PageFunc
	resetPasswordLinkSentPageFunc web.PageFunc
	resetPasswordPageFunc         web.PageFunc
	changePasswordPageFunc        web.PageFunc
	totpSetupPageFunc             web.PageFunc
	totpValidatePageFunc          web.PageFunc

	notifyUserOfResetPasswordLinkFunc NotifyUserOfResetPasswordLinkFunc
	passwordValidationFunc            PasswordValidationFunc

	afterLoginHook                 HookFunc
	afterFailedToLoginHook         HookFunc
	afterUserLockedHook            HookFunc
	afterLogoutHook                HookFunc
	afterSendResetPasswordLinkHook HookFunc
	afterResetPasswordHook         HookFunc
	afterChangePasswordHook        HookFunc
	afterExtendSessionHook         HookFunc
	afterTOTPCodeReusedHook        HookFunc

	db                   *gorm.DB
	userModel            interface{}
	snakePrimaryField    string
	tUser                reflect.Type
	userPassEnabled      bool
	oauthEnabled         bool
	sessionSecureEnabled bool
	totpEnabled          bool
	totpIssuer           string

	i18nBuilder *i18n.Builder
}

func New() *Builder {
	r := &Builder{
		authCookieName:        "auth",
		authSecureCookieName:  "qor5_auth_secure",
		continueUrlCookieName: "qor5_continue_url",

		homePageURLFunc: func(r *http.Request, user interface{}) string {
			return "/"
		},
		loginPageURL: "/auth/login",
		LogoutURL:    "/auth/logout",

		validateTOTPURL:     "/auth/2fa/totp/do",
		totpSetupPageURL:    "/auth/2fa/totp/setup",
		totpValidatePageURL: "/auth/2fa/totp/validate",

		oauthBeginURL:            "/auth/begin",
		oauthCallbackURL:         "/auth/callback",
		oauthCallbackCompleteURL: "/auth/callback-complete",

		passwordLoginURL:             "/auth/userpass/login",
		resetPasswordURL:             "/auth/do-reset-password",
		resetPasswordPageURL:         "/auth/reset-password",
		changePasswordURL:            "/auth/do-change-password",
		changePasswordPageURL:        "/auth/change-password",
		forgetPasswordPageURL:        "/auth/forget-password",
		sendResetPasswordLinkURL:     "/auth/send-reset-password-link",
		resetPasswordLinkSentPageURL: "/auth/reset-password-link-sent",

		sessionMaxAge: 60 * 60,
		cookieConfig: CookieConfig{
			Path:     "/",
			Domain:   "",
			SameSite: http.SameSiteStrictMode,
		},
		autoExtendSession: true,
		maxRetryCount:     5,
		totpEnabled:       true,
		totpIssuer:        "qor5",
		i18nBuilder:       i18n.New(),
	}

	r.registerI18n()
	r.initAllowURLs()

	vh := r.ViewHelper()
	r.loginPageFunc = defaultLoginPage(vh)
	r.forgetPasswordPageFunc = defaultForgetPasswordPage(vh)
	r.resetPasswordLinkSentPageFunc = defaultResetPasswordLinkSentPage(vh)
	r.resetPasswordPageFunc = defaultResetPasswordPage(vh)
	r.changePasswordPageFunc = defaultChangePasswordPage(vh)
	r.totpSetupPageFunc = defaultTOTPSetupPage(vh)
	r.totpValidatePageFunc = defaultTOTPValidatePage(vh)

	return r
}

func (b *Builder) initAllowURLs() {
	b.allowURLs = map[string]void{
		b.oauthBeginURL:                {},
		b.oauthCallbackURL:             {},
		b.oauthCallbackCompleteURL:     {},
		b.passwordLoginURL:             {},
		b.forgetPasswordPageURL:        {},
		b.sendResetPasswordLinkURL:     {},
		b.resetPasswordLinkSentPageURL: {},
		b.resetPasswordURL:             {},
		b.resetPasswordPageURL:         {},
		b.validateTOTPURL:              {},
	}
}

func (b *Builder) AllowURL(v string) {
	b.allowURLs[v] = void{}
}

func (b *Builder) Secret(v string) (r *Builder) {
	b.secret = v
	return b
}

func (b *Builder) CookieConfig(v CookieConfig) (r *Builder) {
	b.cookieConfig = v
	return b
}

// RecaptchaConfig should be set if you want to enable Google reCAPTCHA.
func (b *Builder) RecaptchaConfig(v RecaptchaConfig) (r *Builder) {
	b.recaptchaConfig = v
	b.recaptchaEnabled = b.recaptchaConfig.SiteKey != "" && b.recaptchaConfig.SecretKey != ""
	return b
}

func (b *Builder) OAuthProviders(vs ...*Provider) (r *Builder) {
	if len(vs) == 0 {
		return b
	}
	b.oauthEnabled = true
	b.providers = vs
	var gothProviders []goth.Provider
	for _, v := range vs {
		gothProviders = append(gothProviders, v.Goth)
	}
	goth.UseProviders(gothProviders...)
	return b
}

func (b *Builder) AuthCookieName(v string) (r *Builder) {
	b.authCookieName = v
	return b
}

func (b *Builder) LoginURL(v string) (r *Builder) {
	b.loginPageURL = v
	return b
}

func (b *Builder) HomeURLFunc(v HomeURLFunc) (r *Builder) {
	b.homePageURLFunc = v
	return b
}

func (b *Builder) LoginPageFunc(v web.PageFunc) (r *Builder) {
	b.loginPageFunc = v
	return b
}

func (b *Builder) ForgetPasswordPageFunc(v web.PageFunc) (r *Builder) {
	b.forgetPasswordPageFunc = v
	return b
}

func (b *Builder) ResetPasswordLinkSentPageFunc(v web.PageFunc) (r *Builder) {
	b.resetPasswordLinkSentPageFunc = v
	return b
}

func (b *Builder) ResetPasswordPageFunc(v web.PageFunc) (r *Builder) {
	b.resetPasswordPageFunc = v
	return b
}

func (b *Builder) ChangePasswordPageFunc(v web.PageFunc) (r *Builder) {
	b.changePasswordPageFunc = v
	return b
}

func (b *Builder) TOTPSetupPageFunc(v web.PageFunc) (r *Builder) {
	b.totpSetupPageFunc = v
	return b
}

func (b *Builder) TOTPValidatePageFunc(v web.PageFunc) (r *Builder) {
	b.totpValidatePageFunc = v
	return b
}

func (b *Builder) NotifyUserOfResetPasswordLinkFunc(v NotifyUserOfResetPasswordLinkFunc) (r *Builder) {
	b.notifyUserOfResetPasswordLinkFunc = v
	return b
}

func (b *Builder) PasswordValidationFunc(v PasswordValidationFunc) (r *Builder) {
	b.passwordValidationFunc = v
	return b
}

func (b *Builder) wrapHook(v HookFunc) HookFunc {
	if v == nil {
		return nil
	}

	return func(r *http.Request, user interface{}, vals ...interface{}) error {
		if GetCurrentUser(r) == nil {
			r = r.WithContext(context.WithValue(r.Context(), UserKey, user))
		}
		return v(r, user, vals...)
	}
}

func (b *Builder) AfterLogin(v HookFunc) (r *Builder) {
	b.afterLoginHook = b.wrapHook(v)
	return b
}

func (b *Builder) AfterFailedToLogin(v HookFunc) (r *Builder) {
	b.afterFailedToLoginHook = b.wrapHook(v)
	return b
}

func (b *Builder) AfterUserLocked(v HookFunc) (r *Builder) {
	b.afterUserLockedHook = b.wrapHook(v)
	return b
}

func (b *Builder) AfterLogout(v HookFunc) (r *Builder) {
	b.afterLogoutHook = b.wrapHook(v)
	return b
}

func (b *Builder) AfterSendResetPasswordLink(v HookFunc) (r *Builder) {
	b.afterSendResetPasswordLinkHook = b.wrapHook(v)
	return b
}

func (b *Builder) AfterResetPassword(v HookFunc) (r *Builder) {
	b.afterResetPasswordHook = b.wrapHook(v)
	return b
}

func (b *Builder) AfterChangePassword(v HookFunc) (r *Builder) {
	b.afterChangePasswordHook = b.wrapHook(v)
	return b
}

// vals:
// - old session token
func (b *Builder) AfterExtendSession(v HookFunc) (r *Builder) {
	b.afterExtendSessionHook = b.wrapHook(v)
	return b
}

func (b *Builder) AfterTOTPCodeReused(v HookFunc) (r *Builder) {
	b.afterTOTPCodeReusedHook = b.wrapHook(v)
	return b
}

// seconds
// default 1h
func (b *Builder) SessionMaxAge(v int) (r *Builder) {
	b.sessionMaxAge = v
	return b
}

// extend the session if successfully authenticated
// default true
func (b *Builder) AutoExtendSession(v bool) (r *Builder) {
	b.autoExtendSession = v
	return b
}

// default 5
// MaxRetryCount <= 0 means no max retry count limit
func (b *Builder) MaxRetryCount(v int) (r *Builder) {
	b.maxRetryCount = v
	return b
}

func (b *Builder) TOTPEnabled(v bool) (r *Builder) {
	b.totpEnabled = v
	return b
}

func (b *Builder) TOTPIssuer(v string) (r *Builder) {
	b.totpIssuer = v
	return b
}

func (b *Builder) NoForgetPasswordLink(v bool) (r *Builder) {
	b.noForgetPasswordLink = v
	return b
}

func (b *Builder) DB(v *gorm.DB) (r *Builder) {
	b.db = v
	return b
}

func (b *Builder) I18n(v *i18n.Builder) (r *Builder) {
	b.i18nBuilder = v
	b.registerI18n()
	return b
}

func (b *Builder) GetSessionMaxAge() int {
	return b.sessionMaxAge
}

func (b *Builder) ViewHelper() *ViewHelper {
	return &ViewHelper{
		b: b,
	}
}

func (b *Builder) registerI18n() {
	b.i18nBuilder.RegisterForModule(language.English, I18nLoginKey, Messages_en_US).
		RegisterForModule(language.SimplifiedChinese, I18nLoginKey, Messages_zh_CN)
}

func (b *Builder) UserModel(m interface{}) (r *Builder) {
	b.userModel = m
	b.tUser = underlyingReflectType(reflect.TypeOf(m))
	b.snakePrimaryField = snakePrimaryField(m)
	if _, ok := m.(UserPasser); ok {
		b.userPassEnabled = true
	}
	if _, ok := m.(OAuthUser); ok {
		b.oauthEnabled = true
	}
	if _, ok := m.(SessionSecurer); ok {
		b.sessionSecureEnabled = true
	}
	return b
}

func (b *Builder) newUserObject() interface{} {
	return reflect.New(b.tUser).Interface()
}

func (b *Builder) findUserByID(id string) (user interface{}, err error) {
	m := b.newUserObject()
	err = b.db.Where(fmt.Sprintf("%s = ?", b.snakePrimaryField), id).
		First(m).
		Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return m, nil
}

// completeUserAuthCallback is for url "/auth/{provider}/callback"
func (b *Builder) completeUserAuthCallback(w http.ResponseWriter, r *http.Request) {
	if b.cookieConfig.SameSite != http.SameSiteStrictMode {
		b.completeUserAuthCallbackComplete(w, r)
		return
	}

	completeURL := fmt.Sprintf("%s?%s", b.oauthCallbackCompleteURL, r.URL.Query().Encode())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(fmt.Sprintf(`
<script>
window.location.href="%s";
</script>
<a href="%s">complete</a>
    `, completeURL, completeURL)))
	return
}

func (b *Builder) completeUserAuthCallbackComplete(w http.ResponseWriter, r *http.Request) {
	var err error
	var user interface{}
	defer func() {
		if b.afterFailedToLoginHook != nil && err != nil && user != nil {
			b.afterFailedToLoginHook(r, user)
		}
	}()

	var ouser goth.User
	ouser, err = gothic.CompleteUserAuth(w, r)
	if err != nil {
		log.Println("completeUserAuthWithSetCookie", err)
		setFailCodeFlash(w, FailCodeCompleteUserAuthFailed)
		http.Redirect(w, r, b.LogoutURL, http.StatusFound)
		return
	}

	userID := ouser.UserID

	if b.userModel != nil {
		user, err = b.userModel.(OAuthUser).FindUserByOAuthUserID(b.db, b.newUserObject(), ouser.Provider, ouser.UserID)
		if err != nil {
			if err != gorm.ErrRecordNotFound {
				setFailCodeFlash(w, FailCodeSystemError)
				http.Redirect(w, r, b.LogoutURL, http.StatusFound)
				return
			}
			// TODO: maybe the indentifier of some providers is not email
			indentifier := ouser.Email
			user, err = b.userModel.(OAuthUser).FindUserByOAuthIndentifier(b.db, b.newUserObject(), ouser.Provider, indentifier)
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					setFailCodeFlash(w, FailCodeUserNotFound)
				} else {
					setFailCodeFlash(w, FailCodeSystemError)
				}
				http.Redirect(w, r, b.LogoutURL, http.StatusFound)
				return
			}
			err = user.(OAuthUser).InitOAuthUserID(b.db, b.newUserObject(), ouser.Provider, indentifier, ouser.UserID)
			if err != nil {
				setFailCodeFlash(w, FailCodeSystemError)
				http.Redirect(w, r, b.LogoutURL, http.StatusFound)
				return
			}
		}
		userID = objectID(user)
	}

	claims := UserClaims{
		Provider:         ouser.Provider,
		Email:            ouser.Email,
		Name:             ouser.Name,
		UserID:           userID,
		AvatarURL:        ouser.AvatarURL,
		RegisteredClaims: b.genBaseSessionClaim(userID),
	}
	if user == nil {
		user = &claims
	}

	if b.afterLoginHook != nil {
		setCookieForRequest(r, &http.Cookie{Name: b.authCookieName, Value: b.mustGetSessionToken(claims)})
		if herr := b.afterLoginHook(r, user); herr != nil {
			setFailCodeFlash(w, FailCodeSystemError)
			http.Redirect(w, r, b.loginPageURL, http.StatusFound)
			return
		}
	}

	if err := b.setSecureCookiesByClaims(w, user, claims); err != nil {
		setFailCodeFlash(w, FailCodeSystemError)
		http.Redirect(w, r, b.LogoutURL, http.StatusFound)
		return
	}

	redirectURL := b.homePageURLFunc(r, user)
	if v := b.getContinueURL(w, r); v != "" {
		redirectURL = v
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
	return
}

// return user if account exists even if there is an error returned
func (b *Builder) authUserPass(account string, password string) (user interface{}, err error) {
	user, err = b.userModel.(UserPasser).FindUser(b.db, b.newUserObject(), account)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	u := user.(UserPasser)
	if u.GetLocked() {
		return user, ErrUserLocked
	}

	if !u.IsPasswordCorrect(password) {
		if b.maxRetryCount > 0 {
			if err = u.IncreaseRetryCount(b.db, b.newUserObject()); err != nil {
				return user, err
			}
			if u.GetLoginRetryCount() >= b.maxRetryCount {
				if err = u.LockUser(b.db, b.newUserObject()); err != nil {
					return user, err
				}
				return user, ErrUserGetLocked
			}
		}

		return user, ErrWrongPassword
	}

	if u.GetLoginRetryCount() != 0 {
		if err = u.UnlockUser(b.db, b.newUserObject()); err != nil {
			return user, err
		}
	}
	return user, nil
}

func (b *Builder) userpassLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// check reCAPTCHA token
	if b.recaptchaEnabled {
		token := r.FormValue("token")
		if !recaptchaTokenCheck(b, token) {
			setFailCodeFlash(w, FailCodeIncorrectRecaptchaToken)
			http.Redirect(w, r, b.loginPageURL, http.StatusFound)
			return
		}
	}

	var err error
	var user interface{}
	defer func() {
		if b.afterFailedToLoginHook != nil && err != nil && user != nil {
			b.afterFailedToLoginHook(r, user)
		}
	}()

	account := r.FormValue("account")
	password := r.FormValue("password")
	user, err = b.authUserPass(account, password)
	if err != nil {
		if err == ErrUserGetLocked && b.afterUserLockedHook != nil {
			if herr := b.afterUserLockedHook(r, user); herr != nil {
				setFailCodeFlash(w, FailCodeSystemError)
				http.Redirect(w, r, b.loginPageURL, http.StatusFound)
				return
			}
		}

		code := FailCodeSystemError
		switch err {
		case ErrWrongPassword, ErrUserNotFound:
			code = FailCodeIncorrectAccountNameOrPassword
		case ErrUserLocked, ErrUserGetLocked:
			code = FailCodeUserLocked
		}
		setFailCodeFlash(w, code)
		setWrongLoginInputFlash(w, WrongLoginInputFlash{
			Account:  account,
			Password: password,
		})
		http.Redirect(w, r, b.LogoutURL, http.StatusFound)
		return
	}

	u := user.(UserPasser)
	userID := objectID(user)
	claims := UserClaims{
		UserID:           userID,
		PassUpdatedAt:    u.GetPasswordUpdatedAt(),
		RegisteredClaims: b.genBaseSessionClaim(userID),
	}

	if !b.totpEnabled {
		if b.afterLoginHook != nil {
			setCookieForRequest(r, &http.Cookie{Name: b.authCookieName, Value: b.mustGetSessionToken(claims)})
			if herr := b.afterLoginHook(r, user); herr != nil {
				setFailCodeFlash(w, FailCodeSystemError)
				http.Redirect(w, r, b.loginPageURL, http.StatusFound)
				return
			}
		}
	}

	if err = b.setSecureCookiesByClaims(w, user, claims); err != nil {
		setFailCodeFlash(w, FailCodeSystemError)
		http.Redirect(w, r, b.LogoutURL, http.StatusFound)
		return
	}

	if b.totpEnabled {
		if u.GetIsTOTPSetup() {
			http.Redirect(w, r, b.totpValidatePageURL, http.StatusFound)
			return
		}

		var key *otp.Key
		if key, err = totp.Generate(
			totp.GenerateOpts{
				Issuer:      b.totpIssuer,
				AccountName: u.GetAccountName(),
			},
		); err != nil {
			setFailCodeFlash(w, FailCodeSystemError)
			http.Redirect(w, r, b.LogoutURL, http.StatusFound)
			return
		}

		if err = u.SetTOTPSecret(b.db, b.newUserObject(), key.Secret()); err != nil {
			setFailCodeFlash(w, FailCodeSystemError)
			http.Redirect(w, r, b.LogoutURL, http.StatusFound)
			return
		}

		http.Redirect(w, r, b.totpSetupPageURL, http.StatusFound)
		return
	}

	redirectURL := b.homePageURLFunc(r, user)
	if v := b.getContinueURL(w, r); v != "" {
		redirectURL = v
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
	return
}

func (b *Builder) genBaseSessionClaim(id string) jwt.RegisteredClaims {
	return genBaseClaims(id, b.sessionMaxAge)
}

func (b *Builder) mustGetSessionToken(claims UserClaims) string {
	return mustSignClaims(claims, b.secret)
}

func (b *Builder) setAuthCookiesFromUserClaims(w http.ResponseWriter, claims *UserClaims, secureSalt string) error {
	http.SetCookie(w, &http.Cookie{
		Name:     b.authCookieName,
		Value:    b.mustGetSessionToken(*claims),
		Path:     b.cookieConfig.Path,
		Domain:   b.cookieConfig.Domain,
		MaxAge:   b.sessionMaxAge,
		Expires:  time.Now().Add(time.Duration(b.sessionMaxAge) * time.Second),
		HttpOnly: true,
		Secure:   true,
		SameSite: b.cookieConfig.SameSite,
	})

	if secureSalt != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     b.authSecureCookieName,
			Value:    mustSignClaims(&claims.RegisteredClaims, b.secret+secureSalt),
			Path:     b.cookieConfig.Path,
			Domain:   b.cookieConfig.Domain,
			MaxAge:   b.sessionMaxAge,
			Expires:  time.Now().Add(time.Duration(b.sessionMaxAge) * time.Second),
			HttpOnly: true,
			Secure:   true,
			SameSite: b.cookieConfig.SameSite,
		})
	}

	return nil
}

func (b *Builder) cleanAuthCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     b.authCookieName,
		Value:    "",
		Path:     b.cookieConfig.Path,
		Domain:   b.cookieConfig.Domain,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HttpOnly: true,
		Secure:   true,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     b.authSecureCookieName,
		Value:    "",
		Path:     b.cookieConfig.Path,
		Domain:   b.cookieConfig.Domain,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HttpOnly: true,
		Secure:   true,
	})
}

func (b *Builder) setContinueURL(w http.ResponseWriter, r *http.Request) {
	continueURL := r.RequestURI
	if strings.Contains(continueURL, "?__execute_event__=") {
		continueURL = r.Referer()
	}
	if !strings.HasPrefix(continueURL, "/auth/") {
		http.SetCookie(w, &http.Cookie{
			Name:     b.continueUrlCookieName,
			Value:    continueURL,
			Path:     b.cookieConfig.Path,
			Domain:   b.cookieConfig.Domain,
			HttpOnly: true,
		})
	}
}

func (b *Builder) getContinueURL(w http.ResponseWriter, r *http.Request) string {
	c, err := r.Cookie(b.continueUrlCookieName)
	if err != nil || c.Value == "" {
		return ""
	}

	http.SetCookie(w, &http.Cookie{
		Name:     b.continueUrlCookieName,
		Value:    "",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		Path:     b.cookieConfig.Path,
		Domain:   b.cookieConfig.Domain,
		HttpOnly: true,
	})

	return c.Value
}

func (b *Builder) setSecureCookiesByClaims(w http.ResponseWriter, user interface{}, claims UserClaims) (err error) {
	var secureSalt string
	if b.sessionSecureEnabled {
		if user.(SessionSecurer).GetSecure() == "" {
			err = user.(SessionSecurer).UpdateSecure(b.db, b.newUserObject(), objectID(user))
			if err != nil {
				return err
			}
		}
		secureSalt = user.(SessionSecurer).GetSecure()
	}
	if err = b.setAuthCookiesFromUserClaims(w, &claims, secureSalt); err != nil {
		return err
	}

	return nil
}

func (b *Builder) consumeTOTPCode(r *http.Request, up UserPasser, passcode string) error {
	if !totp.Validate(passcode, up.GetTOTPSecret()) {
		return ErrWrongTOTPCode
	}
	lastCode, usedAt := up.GetLastUsedTOTPCode()
	if usedAt != nil && time.Now().Sub(*usedAt) > 90*time.Second {
		lastCode = ""
	}
	if passcode == lastCode {
		if b.afterTOTPCodeReusedHook != nil {
			if herr := b.afterTOTPCodeReusedHook(r, GetCurrentUser(r)); herr != nil {
				return herr
			}
		}
		return ErrTOTPCodeHasBeenUsed
	}
	if err := up.SetLastUsedTOTPCode(b.db, b.newUserObject(), passcode); err != nil {
		return err
	}
	return nil
}

func (b *Builder) getFailCodeFromTOTPCodeConsumeError(verr error) FailCode {
	fc := FailCodeSystemError
	switch verr {
	case ErrWrongTOTPCode:
		fc = FailCodeIncorrectTOTPCode
	case ErrTOTPCodeHasBeenUsed:
		fc = FailCodeTOTPCodeHasBeenUsed
	}

	return fc
}

// logout is for url "/logout/{provider}"
func (b *Builder) logout(w http.ResponseWriter, r *http.Request) {
	err := gothic.Logout(w, r)
	if err != nil {
		//
	}

	b.cleanAuthCookies(w)

	if b.afterLogoutHook != nil {
		user := GetCurrentUser(r)
		if user != nil {
			if herr := b.afterLogoutHook(r, user); herr != nil {
				setFailCodeFlash(w, FailCodeSystemError)
				http.Redirect(w, r, b.loginPageURL, http.StatusFound)
				return
			}
		}
	}

	http.Redirect(w, r, b.loginPageURL, http.StatusFound)
}

// beginAuth is for url "/auth/{provider}"
func (b *Builder) beginAuth(w http.ResponseWriter, r *http.Request) {
	gothic.BeginAuthHandler(w, r)
}

func (b *Builder) sendResetPasswordLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	failRedirectURL := b.forgetPasswordPageURL

	// check reCAPTCHA token
	if b.recaptchaEnabled {
		token := r.FormValue("token")
		if !recaptchaTokenCheck(b, token) {
			setFailCodeFlash(w, FailCodeIncorrectRecaptchaToken)
			http.Redirect(w, r, failRedirectURL, http.StatusFound)
			return
		}
	}

	account := strings.TrimSpace(r.FormValue("account"))
	passcode := r.FormValue("otp")
	doTOTP := r.URL.Query().Get("totp") == "1"

	if doTOTP {
		failRedirectURL = MustSetQuery(failRedirectURL, "totp", "1")
	}

	if account == "" {
		setFailCodeFlash(w, FailCodeAccountIsRequired)
		http.Redirect(w, r, failRedirectURL, http.StatusFound)
		return
	}

	u, err := b.userModel.(UserPasser).FindUser(b.db, b.newUserObject(), account)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			setFailCodeFlash(w, FailCodeUserNotFound)
		} else {
			setFailCodeFlash(w, FailCodeSystemError)
		}
		setWrongForgetPasswordInputFlash(w, WrongForgetPasswordInputFlash{
			Account: account,
		})
		http.Redirect(w, r, failRedirectURL, http.StatusFound)
		return
	}

	_, createdAt, _ := u.(UserPasser).GetResetPasswordToken()
	if createdAt != nil {
		v := 60 - int(time.Now().Sub(*createdAt).Seconds())
		if v > 0 {
			setSecondsToRedoFlash(w, v)
			setWrongForgetPasswordInputFlash(w, WrongForgetPasswordInputFlash{
				Account: account,
			})
			http.Redirect(w, r, failRedirectURL, http.StatusFound)
			return
		}
	}

	if u.(UserPasser).GetIsTOTPSetup() {
		if !doTOTP {
			setWrongForgetPasswordInputFlash(w, WrongForgetPasswordInputFlash{
				Account: account,
			})
			failRedirectURL = MustSetQuery(failRedirectURL, "totp", "1")
			http.Redirect(w, r, failRedirectURL, http.StatusFound)
			return
		}

		if err = b.consumeTOTPCode(r, u.(UserPasser), passcode); err != nil {
			fc := b.getFailCodeFromTOTPCodeConsumeError(err)
			setFailCodeFlash(w, fc)
			setWrongForgetPasswordInputFlash(w, WrongForgetPasswordInputFlash{
				Account: account,
				TOTP:    passcode,
			})
			http.Redirect(w, r, failRedirectURL, http.StatusFound)
			return
		}
	}

	token, err := u.(UserPasser).GenerateResetPasswordToken(b.db, b.newUserObject())
	if err != nil {
		setFailCodeFlash(w, FailCodeSystemError)
		setWrongForgetPasswordInputFlash(w, WrongForgetPasswordInputFlash{
			Account: account,
		})
		http.Redirect(w, r, failRedirectURL, http.StatusFound)
		return
	}

	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	link := fmt.Sprintf("%s://%s%s?id=%s&token=%s", scheme, r.Host, b.resetPasswordPageURL, objectID(u), token)
	if doTOTP {
		link = MustSetQuery(link, "totp", "1")
	}
	if err = b.notifyUserOfResetPasswordLinkFunc(u, link); err != nil {
		setFailCodeFlash(w, FailCodeSystemError)
		setWrongForgetPasswordInputFlash(w, WrongForgetPasswordInputFlash{
			Account: account,
		})
		http.Redirect(w, r, failRedirectURL, http.StatusFound)
		return
	}

	if b.afterSendResetPasswordLinkHook != nil {
		if herr := b.afterSendResetPasswordLinkHook(r, u); herr != nil {
			setFailCodeFlash(w, FailCodeSystemError)
			http.Redirect(w, r, failRedirectURL, http.StatusFound)
			return
		}
	}

	http.Redirect(w, r, fmt.Sprintf("%s?a=%s", b.resetPasswordLinkSentPageURL, account), http.StatusFound)
	return
}

func (b *Builder) doResetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	userID := r.FormValue("user_id")
	token := r.FormValue("token")
	passcode := r.FormValue("otp")
	doTOTP := r.URL.Query().Get("totp") == "1"

	failRedirectURL := fmt.Sprintf("%s?id=%s&token=%s", b.resetPasswordPageURL, userID, token)
	if doTOTP {
		failRedirectURL = MustSetQuery(failRedirectURL, "totp", "1")
	}
	if userID == "" {
		setFailCodeFlash(w, FailCodeUserNotFound)
		http.Redirect(w, r, failRedirectURL, http.StatusFound)
		return
	}
	if token == "" {
		setFailCodeFlash(w, FailCodeInvalidToken)
		http.Redirect(w, r, failRedirectURL, http.StatusFound)
		return
	}

	password := r.FormValue("password")
	confirmPassword := r.FormValue("confirm_password")
	if password == "" {
		setFailCodeFlash(w, FailCodePasswordCannotBeEmpty)
		http.Redirect(w, r, failRedirectURL, http.StatusFound)
		return
	}
	if confirmPassword != password {
		setFailCodeFlash(w, FailCodePasswordNotMatch)
		setWrongResetPasswordInputFlash(w, WrongResetPasswordInputFlash{
			Password:        password,
			ConfirmPassword: confirmPassword,
		})
		http.Redirect(w, r, failRedirectURL, http.StatusFound)
		return
	}
	if b.passwordValidationFunc != nil {
		msg, ok := b.passwordValidationFunc(password)
		if !ok {
			setCustomErrorMessageFlash(w, msg)
			setWrongResetPasswordInputFlash(w, WrongResetPasswordInputFlash{
				Password:        password,
				ConfirmPassword: confirmPassword,
			})
			http.Redirect(w, r, failRedirectURL, http.StatusFound)
			return
		}
	}

	u, err := b.findUserByID(userID)
	if err != nil {
		if err == ErrUserNotFound {
			setFailCodeFlash(w, FailCodeUserNotFound)
		} else {
			setFailCodeFlash(w, FailCodeSystemError)
		}
		http.Redirect(w, r, failRedirectURL, http.StatusFound)
		return
	}

	storedToken, _, expired := u.(UserPasser).GetResetPasswordToken()
	if expired {
		setFailCodeFlash(w, FailCodeTokenExpired)
		http.Redirect(w, r, failRedirectURL, http.StatusFound)
		return
	}
	if token != storedToken {
		setFailCodeFlash(w, FailCodeInvalidToken)
		http.Redirect(w, r, failRedirectURL, http.StatusFound)
		return
	}

	if u.(UserPasser).GetIsTOTPSetup() {
		if !doTOTP {
			setWrongResetPasswordInputFlash(w, WrongResetPasswordInputFlash{
				Password:        password,
				ConfirmPassword: confirmPassword,
			})
			failRedirectURL = MustSetQuery(failRedirectURL, "totp", "1")
			http.Redirect(w, r, failRedirectURL, http.StatusFound)
			return
		}

		if err = b.consumeTOTPCode(r, u.(UserPasser), passcode); err != nil {
			fc := b.getFailCodeFromTOTPCodeConsumeError(err)
			setFailCodeFlash(w, fc)
			setWrongResetPasswordInputFlash(w, WrongResetPasswordInputFlash{
				Password:        password,
				ConfirmPassword: confirmPassword,
				TOTP:            passcode,
			})
			http.Redirect(w, r, failRedirectURL, http.StatusFound)
			return
		}
	}

	err = u.(UserPasser).ConsumeResetPasswordToken(b.db, b.newUserObject())
	if err != nil {
		setFailCodeFlash(w, FailCodeSystemError)
		http.Redirect(w, r, failRedirectURL, http.StatusFound)
		return
	}

	err = u.(UserPasser).SetPassword(b.db, b.newUserObject(), password)
	if err != nil {
		setFailCodeFlash(w, FailCodeSystemError)
		http.Redirect(w, r, failRedirectURL, http.StatusFound)
		return
	}

	if b.afterResetPasswordHook != nil {
		if herr := b.afterResetPasswordHook(r, u); herr != nil {
			setFailCodeFlash(w, FailCodeSystemError)
			http.Redirect(w, r, failRedirectURL, http.StatusFound)
			return
		}
	}

	setInfoCodeFlash(w, InfoCodePasswordSuccessfullyReset)
	http.Redirect(w, r, b.loginPageURL, http.StatusFound)
	return
}

type ValidationError struct {
	Msg string
}

func (e *ValidationError) Error() string {
	return e.Msg
}

// validationError
// errWrongPassword
// errEmptyPassword
// errPasswordNotMatch
// errWrongTOTPCode
// errTOTPCodeHasBeenUsed
func (b *Builder) ChangePassword(
	r *http.Request,
	oldPassword string,
	password string,
	confirmPassword string,
	otp string,
) error {
	user := GetCurrentUser(r).(UserPasser)

	if !user.IsPasswordCorrect(oldPassword) {
		return ErrWrongPassword
	}

	if password == "" {
		return ErrEmptyPassword
	}
	if confirmPassword != password {
		return ErrPasswordNotMatch
	}
	if b.passwordValidationFunc != nil {
		msg, ok := b.passwordValidationFunc(password)
		if !ok {
			return &ValidationError{Msg: msg}
		}
	}

	if b.totpEnabled {
		if err := b.consumeTOTPCode(r, user, otp); err != nil {
			return err
		}
	}

	err := user.SetPassword(b.db, b.newUserObject(), password)
	if err != nil {
		return err
	}

	if b.afterChangePasswordHook != nil {
		if herr := b.afterChangePasswordHook(r, user); herr != nil {
			return herr
		}
	}

	return nil
}

func (b *Builder) doFormChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	oldPassword := r.FormValue("old_password")
	password := r.FormValue("password")
	confirmPassword := r.FormValue("confirm_password")
	otp := r.FormValue("otp")

	redirectURL := b.changePasswordPageURL

	err := b.ChangePassword(r, oldPassword, password, confirmPassword, otp)
	if err != nil {
		if ve, ok := err.(*ValidationError); ok {
			setCustomErrorMessageFlash(w, ve.Msg)
		} else {
			fc := FailCodeSystemError
			switch err {
			case ErrWrongPassword:
				fc = FailCodeIncorrectPassword
			case ErrEmptyPassword:
				fc = FailCodePasswordCannotBeEmpty
			case ErrPasswordNotMatch:
				fc = FailCodePasswordNotMatch
			case ErrWrongTOTPCode:
				fc = FailCodeIncorrectTOTPCode
			case ErrTOTPCodeHasBeenUsed:
				fc = FailCodeTOTPCodeHasBeenUsed
			}
			setFailCodeFlash(w, fc)
		}

		setWrongChangePasswordInputFlash(w, WrongChangePasswordInputFlash{
			OldPassword:     oldPassword,
			NewPassword:     password,
			ConfirmPassword: confirmPassword,
			TOTP:            otp,
		})
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	setInfoCodeFlash(w, InfoCodePasswordSuccessfullyChanged)
	http.Redirect(w, r, b.loginPageURL, http.StatusFound)
}

func (b *Builder) totpDo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var err error
	var user interface{}
	defer func() {
		if b.afterFailedToLoginHook != nil && err != nil && user != nil {
			b.afterFailedToLoginHook(r, user)
		}
	}()

	var claims *UserClaims
	claims, err = parseUserClaimsFromCookie(r, b.authCookieName, b.secret)
	if err != nil {
		http.Redirect(w, r, b.LogoutURL, http.StatusFound)
		return
	}

	user, err = b.findUserByID(claims.UserID)
	if err != nil {
		if err == ErrUserNotFound {
			setFailCodeFlash(w, FailCodeUserNotFound)
		} else {
			setFailCodeFlash(w, FailCodeSystemError)
		}
		http.Redirect(w, r, b.LogoutURL, http.StatusFound)
		return
	}
	u := user.(UserPasser)

	otp := r.FormValue("otp")
	isTOTPSetup := u.GetIsTOTPSetup()

	if err := b.consumeTOTPCode(r, u, otp); err != nil {
		fc := b.getFailCodeFromTOTPCodeConsumeError(err)
		setFailCodeFlash(w, fc)
		redirectURL := b.totpValidatePageURL
		if !isTOTPSetup {
			redirectURL = b.totpSetupPageURL
		}
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	if !isTOTPSetup {
		if err = u.SetIsTOTPSetup(b.db, b.newUserObject(), true); err != nil {
			setFailCodeFlash(w, FailCodeSystemError)
			http.Redirect(w, r, b.LogoutURL, http.StatusFound)
			return
		}
	}

	claims.TOTPValidated = true
	if b.afterLoginHook != nil {
		setCookieForRequest(r, &http.Cookie{Name: b.authCookieName, Value: b.mustGetSessionToken(*claims)})
		if herr := b.afterLoginHook(r, user); herr != nil {
			setFailCodeFlash(w, FailCodeSystemError)
			http.Redirect(w, r, b.loginPageURL, http.StatusFound)
			return
		}
	}

	err = b.setSecureCookiesByClaims(w, user, *claims)
	if err != nil {
		setFailCodeFlash(w, FailCodeSystemError)
		http.Redirect(w, r, b.LogoutURL, http.StatusFound)
		return
	}

	redirectURL := b.homePageURLFunc(r, user)
	if v := b.getContinueURL(w, r); v != "" {
		redirectURL = v
	}
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (b *Builder) Mount(mux *http.ServeMux) {
	if len(b.secret) == 0 {
		panic("secret is empty")
	}
	if b.userModel != nil {
		if b.db == nil {
			panic("db is required")
		}
	}

	wb := web.New()

	mux.HandleFunc(b.LogoutURL, b.logout)
	mux.Handle(b.loginPageURL, b.i18nBuilder.EnsureLanguage(wb.Page(b.loginPageFunc)))

	if b.userPassEnabled {
		mux.HandleFunc(b.passwordLoginURL, b.userpassLogin)
		mux.HandleFunc(b.resetPasswordURL, b.doResetPassword)
		mux.HandleFunc(b.changePasswordURL, b.doFormChangePassword)
		mux.Handle(b.resetPasswordPageURL, b.i18nBuilder.EnsureLanguage(wb.Page(b.resetPasswordPageFunc)))
		mux.Handle(b.changePasswordPageURL, b.i18nBuilder.EnsureLanguage(wb.Page(b.changePasswordPageFunc)))
		if !b.noForgetPasswordLink {
			mux.HandleFunc(b.sendResetPasswordLinkURL, b.sendResetPasswordLink)
			mux.Handle(b.forgetPasswordPageURL, b.i18nBuilder.EnsureLanguage(wb.Page(b.forgetPasswordPageFunc)))
			mux.Handle(b.resetPasswordLinkSentPageURL, b.i18nBuilder.EnsureLanguage(wb.Page(b.resetPasswordLinkSentPageFunc)))
		}
		if b.totpEnabled {
			mux.HandleFunc(b.validateTOTPURL, b.totpDo)
			mux.Handle(b.totpSetupPageURL, b.i18nBuilder.EnsureLanguage(wb.Page(b.totpSetupPageFunc)))
			mux.Handle(b.totpValidatePageURL, b.i18nBuilder.EnsureLanguage(wb.Page(b.totpValidatePageFunc)))
		}
	}
	if b.oauthEnabled {
		mux.HandleFunc(b.oauthBeginURL, b.beginAuth)
		mux.HandleFunc(b.oauthCallbackURL, b.completeUserAuthCallback)
		mux.HandleFunc(b.oauthCallbackCompleteURL, b.completeUserAuthCallbackComplete)
	}

	// assets
	assetsSubFS, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		panic(err)
	}
	mux.Handle(assetsPathPrefix, http.StripPrefix(assetsPathPrefix, http.FileServer(http.FS(assetsSubFS))))
}
