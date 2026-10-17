package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"
)

const (
	DefaultIdleTimeout     = 2 * time.Hour
	DefaultAbsoluteTimeout = 7 * 24 * time.Hour

	loginEmailLimit     = 5
	loginIPLimit        = 20
	passwordChangeLimit = 5
	limitWindow         = 15 * time.Minute
	tokenLength         = 43 // 32 random bytes in unpadded base64url
)

type Config struct {
	// Secret keys the session token hash; changing it signs everybody out.
	Secret          []byte
	IdleTimeout     time.Duration
	AbsoluteTimeout time.Duration
	Logger          *slog.Logger
	// Now is overridable for tests.
	Now func() time.Time
}

type Service struct {
	store     Store
	cfg       Config
	limiter   *limiter
	dummyHash string
}

type LoginResult struct {
	Token     string
	ExpiresAt time.Time
	User      User
}

func NewService(store Store, cfg Config) (*Service, error) {
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = DefaultIdleTimeout
	}
	if cfg.AbsoluteTimeout <= 0 {
		cfg.AbsoluteTimeout = DefaultAbsoluteTimeout
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	// Verified against when the email is unknown, so response time does not reveal which emails exist.
	dummy, err := HashPassword("timing-equalizer-not-a-real-password")
	if err != nil {
		return nil, err
	}
	return &Service{store: store, cfg: cfg, limiter: newLimiter(limitWindow, cfg.Now), dummyHash: dummy}, nil
}

func (s *Service) Login(ctx context.Context, email, password, ip, userAgent string) (LoginResult, error) {
	email = NormalizeEmail(email)
	ipKey, emailKey := "ip:"+ip, "email:"+email
	if err := s.checkBlocked(ipKey, loginIPLimit); err != nil {
		return LoginResult{}, err
	}
	if err := s.checkBlocked(emailKey, loginEmailLimit); err != nil {
		return LoginResult{}, err
	}
	if email == "" || password == "" || len(password) > 4*MaxPasswordRunes {
		s.limiter.fail(ipKey)
		return LoginResult{}, ErrInvalidCredentials
	}

	user, err := s.store.UserByEmail(ctx, email)
	switch {
	case errors.Is(err, ErrNotFound):
		_, _, _ = VerifyPassword(s.dummyHash, password)
		return LoginResult{}, s.loginFailed(ctx, nil, ipKey, emailKey, ip, "unknown_user")
	case err != nil:
		return LoginResult{}, err
	}
	match, needsRehash, verr := VerifyPassword(user.PasswordHash, password)
	if verr != nil {
		s.cfg.Logger.Error("stored password hash is unusable", "user_id", user.ID)
	}
	if verr != nil || !match {
		return LoginResult{}, s.loginFailed(ctx, &user.ID, ipKey, emailKey, ip, "bad_password")
	}
	if !user.Active {
		return LoginResult{}, s.loginFailed(ctx, &user.ID, ipKey, emailKey, ip, "inactive")
	}

	s.limiter.reset(emailKey)
	if needsRehash {
		if hash, herr := HashPassword(password); herr == nil {
			if serr := s.store.SetPassword(ctx, user.ID, hash, false); serr != nil {
				s.cfg.Logger.Error("password rehash failed", "user_id", user.ID, "error", serr)
			}
		}
	}

	token, err := newToken()
	if err != nil {
		return LoginResult{}, err
	}
	now := s.cfg.Now()
	expires := now.Add(s.cfg.AbsoluteTimeout)
	if _, err := s.store.CreateSession(ctx, NewSession{UserID: user.ID, TokenHash: s.hashToken(token), CreatedAt: now, ExpiresAt: expires, IP: ip, UserAgent: userAgent}); err != nil {
		return LoginResult{}, err
	}
	if err := s.store.TouchLogin(ctx, user.ID, now); err != nil {
		s.cfg.Logger.Error("last login update failed", "user_id", user.ID, "error", err)
	}
	s.audit(ctx, AuditEntry{ActorUserID: &user.ID, Action: "auth.login.success", EntityType: "user", EntityID: &user.ID, After: map[string]any{"ip": ip}})
	return LoginResult{Token: token, ExpiresAt: expires, User: user}, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (Principal, error) {
	if !validTokenShape(token) {
		return Principal{}, ErrUnauthenticated
	}
	now := s.cfg.Now()
	p, err := s.store.PrincipalBySession(ctx, s.hashToken(token), now, now.Add(-s.cfg.IdleTimeout))
	if errors.Is(err, ErrNotFound) {
		return Principal{}, ErrUnauthenticated
	}
	return p, err
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if !validTokenShape(token) {
		return nil
	}
	hash := s.hashToken(token)
	now := s.cfg.Now()
	if p, err := s.store.PrincipalBySession(ctx, hash, now, now.Add(-s.cfg.IdleTimeout)); err == nil {
		s.audit(ctx, AuditEntry{ActorUserID: &p.UserID, Action: "auth.logout", EntityType: "user", EntityID: &p.UserID})
	}
	return s.store.RevokeSession(ctx, hash)
}

// ChangePassword also ends every other session of the user, so a stolen session does not outlive a password change.
func (s *Service) ChangePassword(ctx context.Context, p Principal, current, next, ip string) error {
	key := "pw:" + strconv.FormatInt(p.UserID, 10)
	if err := s.checkBlocked(key, passwordChangeLimit); err != nil {
		return err
	}
	user, err := s.store.UserByID(ctx, p.UserID)
	if err != nil {
		return err
	}
	match, _, verr := VerifyPassword(user.PasswordHash, current)
	if verr != nil || !match {
		s.limiter.fail(key)
		s.audit(ctx, AuditEntry{ActorUserID: &p.UserID, Action: "auth.password.change_failed", EntityType: "user", EntityID: &p.UserID, After: map[string]any{"ip": ip}})
		return ErrWrongCurrentPassword
	}
	if err := ValidatePassword(next, user.Email); err != nil {
		return err
	}
	if next == current {
		return fmt.Errorf("%w: same as the current password", ErrWeakPassword)
	}
	hash, err := HashPassword(next)
	if err != nil {
		return err
	}
	if err := s.store.SetPassword(ctx, p.UserID, hash, true); err != nil {
		return err
	}
	if err := s.store.RevokeOtherSessions(ctx, p.UserID, p.SessionID); err != nil {
		return err
	}
	s.limiter.reset(key)
	s.audit(ctx, AuditEntry{ActorUserID: &p.UserID, Action: "auth.password.changed", EntityType: "user", EntityID: &p.UserID, After: map[string]any{"ip": ip}})
	return nil
}

func (s *Service) checkBlocked(key string, limit int) error {
	if retry, blocked := s.limiter.blocked(key, limit); blocked {
		return &RateLimitedError{RetryAfter: retry}
	}
	return nil
}

func (s *Service) loginFailed(ctx context.Context, userID *int64, ipKey, emailKey, ip, reason string) error {
	s.limiter.fail(ipKey)
	s.limiter.fail(emailKey)
	// The attempted email is not logged: people sometimes type a password into that field.
	s.audit(ctx, AuditEntry{Action: "auth.login.failure", EntityType: "user", EntityID: userID, After: map[string]any{"ip": ip, "reason": reason}})
	return ErrInvalidCredentials
}

// audit never fails the request: an unavailable journal must not lock everybody out, but the error is logged.
func (s *Service) audit(ctx context.Context, e AuditEntry) {
	if err := s.store.Audit(ctx, e); err != nil {
		s.cfg.Logger.Error("audit write failed", "action", e.Action, "error", err)
	}
}

func (s *Service) hashToken(token string) []byte {
	m := hmac.New(sha256.New, s.cfg.Secret)
	m.Write([]byte(token))
	return m.Sum(nil)
}

func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func validTokenShape(t string) bool {
	if len(t) != tokenLength {
		return false
	}
	for i := 0; i < len(t); i++ {
		c := t[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}
