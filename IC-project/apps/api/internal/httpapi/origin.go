package httpapi

import "net/http"

// sameOrigin blocks cross-site state-changing requests. Browsers always send Origin on cross-origin POSTs,
// so a missing header means a non-browser client, which cannot ride on a victim's session cookie.
func sameOrigin(webOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
			default:
				origin := r.Header.Get("Origin")
				if (origin != "" && origin != webOrigin) || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
					writeError(w, http.StatusForbidden, "forbidden_origin", "Запрос с чужого источника отклонён")
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
