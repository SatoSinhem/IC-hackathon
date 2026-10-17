package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ic-group/ic-tender/apps/api/internal/auth"
	"github.com/ic-group/ic-tender/apps/api/internal/auth/authtest"
)

const (
	testOrigin   = "http://localhost:3000"
	testPassword = "correct horse battery"
)

type authFixture struct {
	handler http.Handler
	store   *authtest.MemStore
	h       *authHandlers
	mux     *http.ServeMux
}

func newAuthFixture(t *testing.T, production bool) *authFixture {
	t.Helper()
	hash, err := auth.HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	store := authtest.NewMemStore()
	store.AddUser(auth.User{Email: "admin@ic.kz", PasswordHash: hash, Role: auth.RoleSuperAdmin, Active: true})
	store.AddUser(auth.User{Email: "junior@ic.kz", PasswordHash: hash, Role: auth.RoleJuniorAdmin, Active: true, MustChangePassword: true})
	store.AddUser(auth.User{Email: "finance@ic.kz", PasswordHash: hash, Role: auth.RoleJuniorAdmin, Active: true}, "VIEW_FINANCE", "AWARD_WINNER")
	sub := int64(7)
	store.AddUser(auth.User{Email: "partner@ic.kz", PasswordHash: hash, Role: auth.RoleSubcontractor, SubcontractorID: &sub, Active: true})

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	svc, err := auth.NewService(store, auth.Config{Secret: []byte(strings.Repeat("s", 32)), Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	h := &authHandlers{svc: svc, cookie: NewCookieConfig(production), logger: logger, now: time.Now}
	mux := http.NewServeMux()
	h.register(mux)
	// Routes used to check the guards, standing in for the future business endpoints.
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "yes"})
	})
	mux.Handle("GET /test/any", h.Authenticated(false, ok))
	mux.Handle("GET /test/admin", h.Authenticated(false, RequireRoles(auth.RoleSuperAdmin, auth.RoleJuniorAdmin)(ok)))
	mux.Handle("GET /test/super", h.Authenticated(false, RequireRoles(auth.RoleSuperAdmin)(ok)))
	return &authFixture{handler: sameOrigin(testOrigin)(mux), store: store, h: h, mux: mux}
}

func (f *authFixture) do(method, path, body string, mutate func(*http.Request)) *httptest.ResponseRecorder {
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	req.RemoteAddr = "192.0.2.10:5555"
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if mutate != nil {
		mutate(req)
	}
	rr := httptest.NewRecorder()
	f.handler.ServeHTTP(rr, req)
	return rr
}

func (f *authFixture) login(t *testing.T, email string) *http.Cookie {
	t.Helper()
	rr := f.do(http.MethodPost, "/api/v1/auth/login", `{"email":"`+email+`","password":"`+testPassword+`"}`, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("login %s: %d %s", email, rr.Code, rr.Body.String())
	}
	for _, c := range rr.Result().Cookies() {
		if c.Name == f.h.cookie.Name {
			return c
		}
	}
	t.Fatal("no session cookie in the login response")
	return nil
}

func withCookie(c *http.Cookie) func(*http.Request) { return func(r *http.Request) { r.AddCookie(c) } }

func errorCode(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	var b errorBody
	if err := json.Unmarshal(rr.Body.Bytes(), &b); err != nil {
		t.Fatalf("not an error body: %q", rr.Body.String())
	}
	return b.Error.Code
}

func TestLoginSetsHardenedCookieAndHidesSecrets(t *testing.T) {
	f := newAuthFixture(t, false)
	rr := f.do(http.MethodPost, "/api/v1/auth/login", `{"email":"Admin@ic.kz","password":"`+testPassword+`"}`, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	c := rr.Result().Cookies()[0]
	if c.Name != "ic_session" || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.MaxAge <= 0 || len(c.Value) != 43 {
		t.Fatalf("weak cookie: %+v", c)
	}
	if rr.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("auth responses must not be cached")
	}
	body := rr.Body.String()
	for _, leaked := range []string{testPassword, "argon2", "password_hash", "passwordHash", c.Value} {
		if strings.Contains(body, leaked) {
			t.Fatalf("response leaks %q: %s", leaked, body)
		}
	}
	var out struct{ User userDTO }
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil || out.User.Email != "admin@ic.kz" || out.User.Role != auth.RoleSuperAdmin {
		t.Fatalf("unexpected user %+v (%v)", out, err)
	}
}

func TestProductionCookieUsesHostPrefixAndSecure(t *testing.T) {
	f := newAuthFixture(t, true)
	c := f.login(t, "admin@ic.kz")
	if c.Name != "__Host-ic_session" || !c.Secure || c.Domain != "" || c.Path != "/" {
		t.Fatalf("production cookie must satisfy the __Host- rules: %+v", c)
	}
}

func TestMeAndLogoutFlow(t *testing.T) {
	f := newAuthFixture(t, false)
	if rr := f.do(http.MethodGet, "/api/v1/auth/me", "", nil); rr.Code != http.StatusUnauthorized || errorCode(t, rr) != "unauthenticated" {
		t.Fatalf("anonymous /me: %d %s", rr.Code, rr.Body.String())
	}
	c := f.login(t, "admin@ic.kz")
	if rr := f.do(http.MethodGet, "/api/v1/auth/me", "", withCookie(c)); rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "admin@ic.kz") {
		t.Fatalf("/me: %d %s", rr.Code, rr.Body.String())
	}
	rr := f.do(http.MethodPost, "/api/v1/auth/logout", "", withCookie(c))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("logout: %d", rr.Code)
	}
	cleared := rr.Result().Cookies()
	if len(cleared) == 0 || cleared[0].MaxAge >= 0 {
		t.Fatalf("logout must clear the cookie: %+v", cleared)
	}
	if rr := f.do(http.MethodGet, "/api/v1/auth/me", "", withCookie(c)); rr.Code != http.StatusUnauthorized {
		t.Fatalf("the old cookie must stop working after logout, got %d", rr.Code)
	}
}

func TestMeReportsSortedPermissionsOnlyForGrantedUsers(t *testing.T) {
	f := newAuthFixture(t, false)
	c := f.login(t, "finance@ic.kz")
	rr := f.do(http.MethodGet, "/api/v1/auth/me", "", withCookie(c))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"permissions":["AWARD_WINNER","VIEW_FINANCE"]`) {
		t.Fatalf("permissions must be listed in order: %d %s", rr.Code, rr.Body.String())
	}
	plain := f.do(http.MethodGet, "/api/v1/auth/me", "", withCookie(f.login(t, "admin@ic.kz")))
	if strings.Contains(plain.Body.String(), "permissions") {
		t.Fatalf("a user without grants must not get an empty list: %s", plain.Body.String())
	}
}

func TestInvalidCredentialsMessageDoesNotRevealWhichPartIsWrong(t *testing.T) {
	f := newAuthFixture(t, false)
	a := f.do(http.MethodPost, "/api/v1/auth/login", `{"email":"nobody@ic.kz","password":"`+testPassword+`"}`, nil)
	b := f.do(http.MethodPost, "/api/v1/auth/login", `{"email":"admin@ic.kz","password":"wrong password value"}`, nil)
	if a.Code != http.StatusUnauthorized || b.Code != http.StatusUnauthorized || a.Body.String() != b.Body.String() {
		t.Fatalf("responses differ: %d %q vs %d %q", a.Code, a.Body.String(), b.Code, b.Body.String())
	}
	if len(a.Result().Cookies()) != 0 {
		t.Fatal("a failed login must not set a cookie")
	}
}

func TestLoginRateLimitReturns429WithRetryAfter(t *testing.T) {
	f := newAuthFixture(t, false)
	bad := `{"email":"admin@ic.kz","password":"wrong password value"}`
	for i := 0; i < 5; i++ {
		f.do(http.MethodPost, "/api/v1/auth/login", bad, nil)
	}
	rr := f.do(http.MethodPost, "/api/v1/auth/login", `{"email":"admin@ic.kz","password":"`+testPassword+`"}`, nil)
	if rr.Code != http.StatusTooManyRequests || errorCode(t, rr) != "rate_limited" || rr.Header().Get("Retry-After") == "" {
		t.Fatalf("expected 429 with Retry-After, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestCrossSiteStateChangesAreRejected(t *testing.T) {
	f := newAuthFixture(t, false)
	c := f.login(t, "admin@ic.kz")
	cases := map[string]func(*http.Request){
		"foreign origin":            func(r *http.Request) { r.AddCookie(c); r.Header.Set("Origin", "https://evil.example") },
		"sec-fetch-site cross-site": func(r *http.Request) { r.AddCookie(c); r.Header.Set("Sec-Fetch-Site", "cross-site") },
		"null origin":               func(r *http.Request) { r.AddCookie(c); r.Header.Set("Origin", "null") },
	}
	for name, m := range cases {
		rr := f.do(http.MethodPost, "/api/v1/auth/logout", "", m)
		if rr.Code != http.StatusForbidden || errorCode(t, rr) != "forbidden_origin" {
			t.Errorf("%s: %d %s", name, rr.Code, rr.Body.String())
		}
	}
	if rr := f.do(http.MethodGet, "/api/v1/auth/me", "", withCookie(c)); rr.Code != http.StatusOK {
		t.Fatalf("the blocked logouts must not have ended the session, got %d", rr.Code)
	}
	ok := f.do(http.MethodPost, "/api/v1/auth/logout", "", func(r *http.Request) {
		r.AddCookie(c)
		r.Header.Set("Origin", testOrigin)
		r.Header.Set("Sec-Fetch-Site", "same-origin")
	})
	if ok.Code != http.StatusNoContent {
		t.Fatalf("same-origin request must pass, got %d", ok.Code)
	}
}

func TestRequestBodyValidation(t *testing.T) {
	f := newAuthFixture(t, false)
	cases := []struct {
		name, body, contentType string
		want                    int
	}{
		{"wrong content type", `{"email":"a@b.kz","password":"x"}`, "text/plain", http.StatusUnsupportedMediaType},
		{"unknown field", `{"email":"a@b.kz","password":"x","role":"SUPER_ADMIN"}`, "application/json", http.StatusBadRequest},
		{"trailing data", `{"email":"a@b.kz","password":"x"}{"x":1}`, "application/json", http.StatusBadRequest},
		{"not json", `email=a`, "application/json", http.StatusBadRequest},
		{"too large", `{"email":"a@b.kz","password":"` + strings.Repeat("x", 5000) + `"}`, "application/json", http.StatusBadRequest},
		{"charset parameter is fine", `{"email":"a@b.kz","password":"x"}`, "application/json; charset=utf-8", http.StatusUnauthorized},
	}
	for _, tc := range cases {
		rr := f.do(http.MethodPost, "/api/v1/auth/login", tc.body, func(r *http.Request) { r.Header.Set("Content-Type", tc.contentType) })
		if rr.Code != tc.want {
			t.Errorf("%s: got %d want %d (%s)", tc.name, rr.Code, tc.want, rr.Body.String())
		}
	}
}

func TestMustChangePasswordGate(t *testing.T) {
	f := newAuthFixture(t, false)
	c := f.login(t, "junior@ic.kz")
	if rr := f.do(http.MethodGet, "/test/any", "", withCookie(c)); rr.Code != http.StatusForbidden || errorCode(t, rr) != "password_change_required" {
		t.Fatalf("business endpoints must be locked until the password is changed: %d %s", rr.Code, rr.Body.String())
	}
	if rr := f.do(http.MethodGet, "/api/v1/auth/me", "", withCookie(c)); rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"mustChangePassword":true`) {
		t.Fatalf("/me must stay reachable and report the flag: %d %s", rr.Code, rr.Body.String())
	}
	rr := f.do(http.MethodPost, "/api/v1/auth/change-password", `{"currentPassword":"`+testPassword+`","newPassword":"a brand new passphrase"}`, withCookie(c))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("change-password: %d %s", rr.Code, rr.Body.String())
	}
	if rr := f.do(http.MethodGet, "/test/any", "", withCookie(c)); rr.Code != http.StatusOK {
		t.Fatalf("endpoints must open after the change, got %d", rr.Code)
	}
}

func TestChangePasswordErrors(t *testing.T) {
	f := newAuthFixture(t, false)
	c := f.login(t, "admin@ic.kz")
	wrong := f.do(http.MethodPost, "/api/v1/auth/change-password", `{"currentPassword":"not it at all!!","newPassword":"a brand new passphrase"}`, withCookie(c))
	weak := f.do(http.MethodPost, "/api/v1/auth/change-password", `{"currentPassword":"`+testPassword+`","newPassword":"short"}`, withCookie(c))
	anon := f.do(http.MethodPost, "/api/v1/auth/change-password", `{"currentPassword":"x","newPassword":"y"}`, nil)
	if wrong.Code != http.StatusBadRequest || errorCode(t, wrong) != "wrong_current_password" ||
		weak.Code != http.StatusUnprocessableEntity || errorCode(t, weak) != "weak_password" || anon.Code != http.StatusUnauthorized {
		t.Fatalf("wrong=%d weak=%d anon=%d", wrong.Code, weak.Code, anon.Code)
	}
}

func TestRoleGuards(t *testing.T) {
	f := newAuthFixture(t, false)
	sup := f.login(t, "admin@ic.kz")
	part := f.login(t, "partner@ic.kz")

	cases := []struct {
		name   string
		path   string
		cookie *http.Cookie
		want   int
	}{
		{"anonymous on protected route", "/test/any", nil, http.StatusUnauthorized},
		{"anonymous on admin route", "/test/admin", nil, http.StatusUnauthorized},
		{"subcontractor on admin route", "/test/admin", part, http.StatusForbidden},
		{"subcontractor on super route", "/test/super", part, http.StatusForbidden},
		{"subcontractor on common route", "/test/any", part, http.StatusOK},
		{"super admin on admin route", "/test/admin", sup, http.StatusOK},
		{"super admin on super route", "/test/super", sup, http.StatusOK},
	}
	for _, tc := range cases {
		var m func(*http.Request)
		if tc.cookie != nil {
			m = withCookie(tc.cookie)
		}
		if rr := f.do(http.MethodGet, tc.path, "", m); rr.Code != tc.want {
			t.Errorf("%s: got %d want %d", tc.name, rr.Code, tc.want)
		}
	}
}

func TestRevokedSessionClearsCookieOnUse(t *testing.T) {
	f := newAuthFixture(t, false)
	rr := f.do(http.MethodGet, "/test/any", "", func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: "ic_session", Value: strings.Repeat("A", 43)})
	})
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", rr.Code)
	}
	cs := rr.Result().Cookies()
	if len(cs) == 0 || cs[0].MaxAge >= 0 {
		t.Fatalf("an invalid cookie must be cleared: %+v", cs)
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.5:4444"
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 198.51.100.7")
	if got := clientIP(r, false); got != "203.0.113.5" {
		t.Fatalf("untrusted proxy: %s", got)
	}
	if got := clientIP(r, true); got != "198.51.100.7" {
		t.Fatalf("trusted proxy must use the last entry: %s", got)
	}
	r.Header.Set("X-Forwarded-For", "not-an-ip")
	if got := clientIP(r, true); got != "203.0.113.5" {
		t.Fatalf("garbage header must fall back to the peer: %s", got)
	}
	r.RemoteAddr = "[2001:db8::1]:9"
	r.Header.Del("X-Forwarded-For")
	if got := clientIP(r, true); got != "2001:db8::1" {
		t.Fatalf("ipv6: %s", got)
	}
}

func TestFullHandlerRoutesAuthAndHealth(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	svc, _ := auth.NewService(authtest.NewMemStore(), auth.Config{Logger: logger})
	h := New(Deps{DB: &fakeDB{}, Auth: svc, WebOrigin: testOrigin, Logger: logger})
	for path, want := range map[string]int{"/api/v1/health": 200, "/api/v1/auth/me": 401} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code != want {
			t.Errorf("%s: got %d want %d", path, rr.Code, want)
		}
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{}`))
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("the global chain must block foreign origins, got %d", rr.Code)
	}
}
