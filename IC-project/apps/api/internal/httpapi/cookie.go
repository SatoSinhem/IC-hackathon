package httpapi

import (
	"net/http"
	"time"
)

// CookieConfig describes the session cookie. In production the __Host- prefix forces Secure, Path=/ and no Domain.
type CookieConfig struct {
	Name   string
	Secure bool
}

func NewCookieConfig(production bool) CookieConfig {
	if production {
		return CookieConfig{Name: "__Host-ic_session", Secure: true}
	}
	return CookieConfig{Name: "ic_session"}
}

func (c CookieConfig) set(w http.ResponseWriter, token string, expires, now time.Time) {
	maxAge := int(expires.Sub(now).Seconds())
	if maxAge < 1 {
		maxAge = 1
	}
	http.SetCookie(w, &http.Cookie{Name: c.Name, Value: token, Path: "/", MaxAge: maxAge, HttpOnly: true, Secure: c.Secure, SameSite: http.SameSiteLaxMode})
}

func (c CookieConfig) clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: c.Name, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: c.Secure, SameSite: http.SameSiteLaxMode})
}

func (c CookieConfig) read(r *http.Request) string {
	ck, err := r.Cookie(c.Name)
	if err != nil {
		return ""
	}
	return ck.Value
}
