package auth

import (
	"context"
	"errors"
	"strings"
	"time"
)

type Role string

const (
	RoleSuperAdmin    Role = "SUPER_ADMIN"
	RoleJuniorAdmin   Role = "JUNIOR_ADMIN"
	RoleSubcontractor Role = "SUBCONTRACTOR"
)

func (r Role) Valid() bool {
	return r == RoleSuperAdmin || r == RoleJuniorAdmin || r == RoleSubcontractor
}

var (
	ErrNotFound             = errors.New("not found")
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrUnauthenticated      = errors.New("unauthenticated")
	ErrWrongCurrentPassword = errors.New("wrong current password")
)

// RateLimitedError reports a temporary lockout together with the remaining wait time.
type RateLimitedError struct{ RetryAfter time.Duration }

func (e *RateLimitedError) Error() string { return "too many attempts" }

type User struct {
	ID                 int64
	Email              string
	PasswordHash       string
	Role               Role
	SubcontractorID    *int64
	MustChangePassword bool
	// Active is false for suspended users and for subcontractor users whose company is not ACTIVE.
	Active bool
}

type Principal struct {
	UserID             int64
	Email              string
	Role               Role
	SubcontractorID    *int64
	SessionID          int64
	MustChangePassword bool
	Permissions        map[string]bool
}

func (p Principal) IsAdmin() bool { return p.Role == RoleSuperAdmin || p.Role == RoleJuniorAdmin }

func (p Principal) HasRole(roles ...Role) bool {
	for _, r := range roles {
		if p.Role == r {
			return true
		}
	}
	return false
}

// Can reports whether an administrator holds a permission; SUPER_ADMIN holds all, subcontractors none.
func (p Principal) Can(permission string) bool {
	if p.Role == RoleSuperAdmin {
		return true
	}
	return p.Role == RoleJuniorAdmin && p.Permissions[permission]
}

type principalKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

func NormalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

type NewSession struct {
	UserID    int64
	TokenHash []byte
	CreatedAt time.Time
	ExpiresAt time.Time
	IP        string
	UserAgent string
}

type AuditEntry struct {
	ActorUserID *int64
	Action      string
	EntityType  string
	EntityID    *int64
	After       map[string]any
	Reason      string
}

type Store interface {
	UserByEmail(ctx context.Context, email string) (User, error)
	UserByID(ctx context.Context, id int64) (User, error)
	CreateSession(ctx context.Context, s NewSession) (int64, error)
	// PrincipalBySession returns ErrNotFound for unknown, revoked, expired or idle sessions and for inactive users.
	PrincipalBySession(ctx context.Context, tokenHash []byte, now, idleCutoff time.Time) (Principal, error)
	RevokeSession(ctx context.Context, tokenHash []byte) error
	RevokeOtherSessions(ctx context.Context, userID, keepSessionID int64) error
	SetPassword(ctx context.Context, userID int64, hash string, clearMustChange bool) error
	TouchLogin(ctx context.Context, userID int64, at time.Time) error
	Audit(ctx context.Context, e AuditEntry) error
}
