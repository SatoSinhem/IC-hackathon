package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PGStore struct{ pool *pgxpool.Pool }

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

const userSelect = `SELECT u.id, u.email, u.password_hash, u.role, u.subcontractor_id, u.must_change_password,
 (u.status = 'ACTIVE' AND (u.subcontractor_id IS NULL OR sc.status = 'ACTIVE'))
FROM users u LEFT JOIN subcontractors sc ON sc.id = u.subcontractor_id `

func scanUser(row pgx.Row) (User, error) {
	var u User
	var role string
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &role, &u.SubcontractorID, &u.MustChangePassword, &u.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	u.Role = Role(role)
	return u, err
}

func (s *PGStore) UserByEmail(ctx context.Context, email string) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, userSelect+`WHERE u.email = $1`, email))
}

func (s *PGStore) UserByID(ctx context.Context, id int64) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, userSelect+`WHERE u.id = $1`, id))
}

func (s *PGStore) CreateSession(ctx context.Context, n NewSession) (int64, error) {
	var ip *string
	if addr, err := netip.ParseAddr(n.IP); err == nil {
		v := addr.String()
		ip = &v
	}
	ua := n.UserAgent
	if len(ua) > 512 {
		ua = ua[:512]
	}
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO user_sessions (user_id, token_hash, created_at, expires_at, last_seen_at, ip_address, user_agent)
		 VALUES ($1, $2, $3, $4, $3, $5::inet, NULLIF($6, '')) RETURNING id`,
		n.UserID, n.TokenHash, n.CreatedAt, n.ExpiresAt, ip, ua).Scan(&id)
	return id, err
}

func (s *PGStore) PrincipalBySession(ctx context.Context, tokenHash []byte, now, idleCutoff time.Time) (Principal, error) {
	var p Principal
	var role string
	var perms []string
	err := s.pool.QueryRow(ctx,
		`SELECT s.id, u.id, u.email, u.role, u.subcontractor_id, u.must_change_password,
		  ARRAY(SELECT up.permission FROM user_permissions up WHERE up.user_id = u.id)
		 FROM user_sessions s
		 JOIN users u ON u.id = s.user_id
		 LEFT JOIN subcontractors sc ON sc.id = u.subcontractor_id
		 WHERE s.token_hash = $1 AND s.revoked_at IS NULL AND s.expires_at > $2
		   AND COALESCE(s.last_seen_at, s.created_at) > $3
		   AND u.status = 'ACTIVE' AND (u.subcontractor_id IS NULL OR sc.status = 'ACTIVE')`,
		tokenHash, now, idleCutoff).Scan(&p.SessionID, &p.UserID, &p.Email, &role, &p.SubcontractorID, &p.MustChangePassword, &perms)
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, ErrNotFound
	}
	if err != nil {
		return Principal{}, err
	}
	p.Role = Role(role)
	p.Permissions = make(map[string]bool, len(perms))
	for _, perm := range perms {
		p.Permissions[perm] = true
	}
	// Idle time is tracked with one-minute resolution to avoid a write on every request.
	_, err = s.pool.Exec(ctx,
		`UPDATE user_sessions SET last_seen_at = $2::timestamptz
		 WHERE id = $1 AND COALESCE(last_seen_at, created_at) < $2::timestamptz - interval '1 minute'`, p.SessionID, now)
	return p, err
}

func (s *PGStore) RevokeSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.pool.Exec(ctx, `UPDATE user_sessions SET revoked_at = NOW() WHERE token_hash = $1 AND revoked_at IS NULL`, tokenHash)
	return err
}

func (s *PGStore) RevokeOtherSessions(ctx context.Context, userID, keepSessionID int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE user_sessions SET revoked_at = NOW() WHERE user_id = $1 AND id <> $2 AND revoked_at IS NULL`, userID, keepSessionID)
	return err
}

func (s *PGStore) SetPassword(ctx context.Context, userID int64, hash string, clearMustChange bool) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET password_hash = $2, must_change_password = CASE WHEN $3 THEN FALSE ELSE must_change_password END, updated_at = NOW() WHERE id = $1`,
		userID, hash, clearMustChange)
	return err
}

func (s *PGStore) TouchLogin(ctx context.Context, userID int64, at time.Time) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET last_login_at = $2 WHERE id = $1`, userID, at)
	return err
}

func (s *PGStore) Audit(ctx context.Context, e AuditEntry) error {
	var after *string
	if e.After != nil {
		b, err := json.Marshal(e.After)
		if err != nil {
			return err
		}
		v := string(b)
		after = &v
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id, after_data, reason) VALUES ($1, $2, $3, $4, $5::jsonb, NULLIF($6, ''))`,
		e.ActorUserID, e.Action, e.EntityType, e.EntityID, after, e.Reason)
	return err
}
