package httpapi

import (
	"errors"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/ic-group/ic-tender/apps/api/internal/auth"
)

type authHandlers struct {
	svc            *auth.Service
	cookie         CookieConfig
	logger         *slog.Logger
	trustForwarded bool
	now            func() time.Time
}

type userDTO struct {
	ID                 int64     `json:"id"`
	Email              string    `json:"email"`
	Role               auth.Role `json:"role"`
	SubcontractorID    *int64    `json:"subcontractorId,omitempty"`
	MustChangePassword bool      `json:"mustChangePassword"`
	// Permissions is filled only by /auth/me; the interface uses it to hide sections, the server still enforces access.
	Permissions []string `json:"permissions,omitempty"`
}

func (h *authHandlers) register(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/auth/login", noStore(http.HandlerFunc(h.login)))
	mux.Handle("POST /api/v1/auth/logout", noStore(http.HandlerFunc(h.logout)))
	mux.Handle("GET /api/v1/auth/me", noStore(h.Authenticated(true, http.HandlerFunc(h.me))))
	mux.Handle("POST /api/v1/auth/change-password", noStore(h.Authenticated(true, http.HandlerFunc(h.changePassword))))
}

// Authenticated loads the session into the request context. Until the first password change only endpoints
// registered with allowPasswordChange are reachable.
func (h *authHandlers) Authenticated(allowPasswordChange bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := h.svc.Authenticate(r.Context(), h.cookie.read(r))
		if err != nil {
			if !errors.Is(err, auth.ErrUnauthenticated) {
				h.logger.Error("session lookup failed", "error", err)
				writeError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка сервера")
				return
			}
			h.cookie.clear(w)
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Требуется вход в систему")
			return
		}
		if p.MustChangePassword && !allowPasswordChange {
			writeError(w, http.StatusForbidden, "password_change_required", "Необходимо сменить пароль")
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), p)))
	})
}

// RequireRoles must be placed behind Authenticated. Every protected endpoint also has to check access to the concrete object.
func RequireRoles(roles ...auth.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := auth.PrincipalFrom(r.Context())
			if !ok || !p.HasRole(roles...) {
				writeError(w, http.StatusForbidden, "forbidden", "Недостаточно прав")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (h *authHandlers) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	res, err := h.svc.Login(r.Context(), in.Email, in.Password, clientIP(r, h.trustForwarded), r.UserAgent())
	if !h.handleError(w, err) {
		return
	}
	h.cookie.set(w, res.Token, res.ExpiresAt, h.now())
	writeJSON(w, http.StatusOK, map[string]any{"user": toDTO(res.User.ID, res.User.Email, res.User.Role, res.User.SubcontractorID, res.User.MustChangePassword)})
}

func (h *authHandlers) logout(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Logout(r.Context(), h.cookie.read(r)); err != nil {
		h.logger.Error("logout failed", "error", err)
	}
	h.cookie.clear(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *authHandlers) me(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFrom(r.Context())
	dto := toDTO(p.UserID, p.Email, p.Role, p.SubcontractorID, p.MustChangePassword)
	for perm := range p.Permissions {
		dto.Permissions = append(dto.Permissions, perm)
	}
	sort.Strings(dto.Permissions)
	writeJSON(w, http.StatusOK, map[string]any{"user": dto})
}

func (h *authHandlers) changePassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	p, _ := auth.PrincipalFrom(r.Context())
	if !h.handleError(w, h.svc.ChangePassword(r.Context(), p, in.CurrentPassword, in.NewPassword, clientIP(r, h.trustForwarded))) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleError writes the response for a service error and reports whether the request may continue.
func (h *authHandlers) handleError(w http.ResponseWriter, err error) bool {
	var limited *auth.RateLimitedError
	switch {
	case err == nil:
		return true
	case errors.As(err, &limited):
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(limited.RetryAfter.Seconds()))))
		writeError(w, http.StatusTooManyRequests, "rate_limited", "Слишком много попыток. Повторите позже.")
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "Неверный email или пароль")
	case errors.Is(err, auth.ErrWrongCurrentPassword):
		writeError(w, http.StatusBadRequest, "wrong_current_password", "Текущий пароль указан неверно")
	case errors.Is(err, auth.ErrWeakPassword):
		writeError(w, http.StatusUnprocessableEntity, "weak_password", "Пароль должен содержать от 12 до 128 символов, не совпадать с email и с текущим паролем")
	default:
		h.logger.Error("auth request failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка сервера")
	}
	return false
}

func toDTO(id int64, email string, role auth.Role, subcontractorID *int64, mustChange bool) userDTO {
	return userDTO{ID: id, Email: email, Role: role, SubcontractorID: subcontractorID, MustChangePassword: mustChange}
}
