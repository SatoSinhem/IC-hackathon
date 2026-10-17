package auth_test

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/argon2"

	"github.com/ic-group/ic-tender/apps/api/internal/auth"
)

var emailSeq atomic.Int64

type pgFixture struct {
	pool  *pgxpool.Pool
	svc   *auth.Service
	clock *fakeClock
	ctx   context.Context
}

// newPGFixture needs a disposable database with all migrations applied; audit_logs is append-only, so test rows cannot be removed.
func newPGFixture(t *testing.T) *pgFixture {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var name string
	if err := pool.QueryRow(ctx, `SELECT current_database()`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(name, "test") {
		t.Skipf("refusing to run against database %q: its name must contain \"test\"", name)
	}
	clock := &fakeClock{t: time.Now().UTC().Truncate(time.Second)}
	svc, err := auth.NewService(auth.NewPGStore(pool), auth.Config{
		Secret: []byte(strings.Repeat("p", 32)), IdleTimeout: 2 * time.Hour, AbsoluteTimeout: 7 * 24 * time.Hour,
		Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)), Now: clock.now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return &pgFixture{pool: pool, svc: svc, clock: clock, ctx: ctx}
}

func (f *pgFixture) addUser(t *testing.T, role auth.Role, subcontractorID *int64, mustChange bool) (id int64, email string) {
	t.Helper()
	hash, err := auth.HashPassword(goodPassword)
	if err != nil {
		t.Fatal(err)
	}
	email = fmt.Sprintf("u%d-%d@test.kz", time.Now().UnixNano(), emailSeq.Add(1))
	err = f.pool.QueryRow(f.ctx,
		`INSERT INTO users (email, password_hash, role, subcontractor_id, must_change_password) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		email, hash, string(role), subcontractorID, mustChange).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id, email
}

func (f *pgFixture) addSubcontractor(t *testing.T, status string) int64 {
	t.Helper()
	var id int64
	err := f.pool.QueryRow(f.ctx, `INSERT INTO subcontractors (legal_name, status) VALUES ($1,$2) RETURNING id`, fmt.Sprintf("Sub %d", time.Now().UnixNano()), status).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (f *pgFixture) login(t *testing.T, email string) auth.LoginResult {
	t.Helper()
	res, err := f.svc.Login(f.ctx, email, goodPassword, "203.0.113.9", "pg-test-agent")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	return res
}

func TestPGSessionLifecycle(t *testing.T) {
	f := newPGFixture(t)
	id, email := f.addUser(t, auth.RoleJuniorAdmin, nil, false)
	if _, err := f.pool.Exec(f.ctx, `INSERT INTO user_permissions (user_id, permission, granted_by) VALUES ($1,'AWARD_WINNER',$1)`, id); err != nil {
		t.Fatal(err)
	}
	res := f.login(t, email)

	p, err := f.svc.Authenticate(f.ctx, res.Token)
	if err != nil || p.UserID != id || p.Role != auth.RoleJuniorAdmin || !p.Can("AWARD_WINNER") || p.Can("OTHER") {
		t.Fatalf("principal %+v err %v", p, err)
	}

	var storedHash []byte
	var ip, ua string
	if err := f.pool.QueryRow(f.ctx, `SELECT token_hash, host(ip_address), user_agent FROM user_sessions WHERE user_id=$1`, id).Scan(&storedHash, &ip, &ua); err != nil {
		t.Fatal(err)
	}
	if len(storedHash) != 32 || strings.Contains(string(storedHash), res.Token) {
		t.Fatal("only a 32-byte keyed hash of the token may be stored")
	}
	if ip != "203.0.113.9" || ua != "pg-test-agent" {
		t.Fatalf("ip=%q ua=%q", ip, ua)
	}

	f.clock.advance(119 * time.Minute)
	if _, err := f.svc.Authenticate(f.ctx, res.Token); err != nil {
		t.Fatalf("within the idle window: %v", err)
	}
	f.clock.advance(119 * time.Minute)
	if _, err := f.svc.Authenticate(f.ctx, res.Token); err != nil {
		t.Fatalf("activity must refresh the idle timer in the database: %v", err)
	}
	f.clock.advance(121 * time.Minute)
	if _, err := f.svc.Authenticate(f.ctx, res.Token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("idle expiry: %v", err)
	}
}

func TestPGAbsoluteExpiryAndLogout(t *testing.T) {
	f := newPGFixture(t)
	_, email := f.addUser(t, auth.RoleSuperAdmin, nil, false)

	res := f.login(t, email)
	f.clock.advance(7*24*time.Hour - time.Minute)
	if _, err := f.svc.Authenticate(f.ctx, res.Token); err == nil {
		// The idle window (2h) has long passed, so this must already be rejected.
		t.Fatal("a session idle for almost 7 days must be rejected")
	}

	res = f.login(t, email)
	for i := 0; i < 7*24; i++ {
		f.clock.advance(time.Hour)
		if _, err := f.svc.Authenticate(f.ctx, res.Token); err != nil {
			if i < 7*24-1 {
				t.Fatalf("hour %d: %v", i, err)
			}
		}
	}
	if _, err := f.svc.Authenticate(f.ctx, res.Token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("absolute expiry: %v", err)
	}

	res = f.login(t, email)
	if err := f.svc.Logout(f.ctx, res.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Authenticate(f.ctx, res.Token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("logout: %v", err)
	}
}

func TestPGSubcontractorMustBeActive(t *testing.T) {
	f := newPGFixture(t)
	sub := f.addSubcontractor(t, "SUSPENDED")
	_, email := f.addUser(t, auth.RoleSubcontractor, &sub, false)

	if _, err := f.svc.Login(f.ctx, email, goodPassword, "203.0.113.9", ""); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("a suspended company must not log in: %v", err)
	}
	if _, err := f.pool.Exec(f.ctx, `UPDATE subcontractors SET status='ACTIVE' WHERE id=$1`, sub); err != nil {
		t.Fatal(err)
	}
	res := f.login(t, email)
	p, err := f.svc.Authenticate(f.ctx, res.Token)
	if err != nil || p.SubcontractorID == nil || *p.SubcontractorID != sub {
		t.Fatalf("principal %+v err %v", p, err)
	}
	if _, err := f.pool.Exec(f.ctx, `UPDATE subcontractors SET status='SUSPENDED' WHERE id=$1`, sub); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Authenticate(f.ctx, res.Token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("suspending the company must end its sessions: %v", err)
	}
}

func TestPGSuspendedUserLosesSession(t *testing.T) {
	f := newPGFixture(t)
	id, email := f.addUser(t, auth.RoleSuperAdmin, nil, false)
	res := f.login(t, email)
	if _, err := f.pool.Exec(f.ctx, `UPDATE users SET status='SUSPENDED' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Authenticate(f.ctx, res.Token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("suspended user: %v", err)
	}
}

func TestPGChangePasswordRevokesOtherSessionsAndAudits(t *testing.T) {
	f := newPGFixture(t)
	id, email := f.addUser(t, auth.RoleJuniorAdmin, nil, true)
	first, second := f.login(t, email), f.login(t, email)
	p, err := f.svc.Authenticate(f.ctx, first.Token)
	if err != nil || !p.MustChangePassword {
		t.Fatalf("principal %+v err %v", p, err)
	}
	if err := f.svc.ChangePassword(f.ctx, p, goodPassword, "a brand new passphrase", "203.0.113.9"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Authenticate(f.ctx, first.Token); err != nil {
		t.Fatalf("own session must survive: %v", err)
	}
	if _, err := f.svc.Authenticate(f.ctx, second.Token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("other session must be revoked: %v", err)
	}
	var must bool
	if err := f.pool.QueryRow(f.ctx, `SELECT must_change_password FROM users WHERE id=$1`, id).Scan(&must); err != nil || must {
		t.Fatalf("flag must be cleared: %v %v", must, err)
	}
	var n int
	if err := f.pool.QueryRow(f.ctx, `SELECT count(*) FROM audit_logs WHERE entity_id=$1 AND action IN ('auth.login.success','auth.password.changed')`, id).Scan(&n); err != nil || n != 3 {
		t.Fatalf("expected 2 logins and 1 password change in the journal, got %d (%v)", n, err)
	}
}

func TestPGFailedLoginIsAuditedWithoutEmail(t *testing.T) {
	f := newPGFixture(t)
	if _, err := f.svc.Login(f.ctx, "ghost-secret@test.kz", "whatever password", "198.51.100.77", ""); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatal(err)
	}
	var n int
	err := f.pool.QueryRow(f.ctx, `SELECT count(*) FROM audit_logs WHERE action='auth.login.failure' AND after_data->>'ip'='198.51.100.77' AND after_data::text NOT LIKE '%ghost-secret%'`).Scan(&n)
	if err != nil || n != 1 {
		t.Fatalf("count=%d err=%v", n, err)
	}
}

func TestPGWeakHashIsUpgradedOnLogin(t *testing.T) {
	f := newPGFixture(t)
	id, email := f.addUser(t, auth.RoleSuperAdmin, nil, true)

	salt := []byte("0123456789abcdef")
	key := argon2.IDKey([]byte(goodPassword), salt, 1, 8*1024, 1, 32)
	legacy := fmt.Sprintf("$argon2id$v=19$m=8192,t=1,p=1$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
	if _, err := f.pool.Exec(f.ctx, `UPDATE users SET password_hash=$2 WHERE id=$1`, id, legacy); err != nil {
		t.Fatal(err)
	}

	f.login(t, email)

	var stored string
	var must bool
	if err := f.pool.QueryRow(f.ctx, `SELECT password_hash, must_change_password FROM users WHERE id=$1`, id).Scan(&stored, &must); err != nil {
		t.Fatal(err)
	}
	if stored == legacy || !strings.HasPrefix(stored, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("the hash must be upgraded to the current parameters, got %q", stored)
	}
	if ok, rehash, err := auth.VerifyPassword(stored, goodPassword); err != nil || !ok || rehash {
		t.Fatalf("the upgraded hash must verify: ok=%v rehash=%v err=%v", ok, rehash, err)
	}
	if !must {
		t.Fatal("an automatic rehash must not clear the must-change flag")
	}
}

func TestPGEmailMustBeNormalized(t *testing.T) {
	f := newPGFixture(t)
	for _, bad := range []string{"Upper@Case.kz", " padded@test.kz", "no-at-sign", "two@@signs.kz", "spa ce@test.kz"} {
		_, err := f.pool.Exec(f.ctx, `INSERT INTO users (email, password_hash, role) VALUES ($1,'x','SUPER_ADMIN')`, bad)
		if err == nil {
			t.Errorf("email %q must be rejected by the database", bad)
		}
	}
}

func TestPGAuditJournalIsAppendOnly(t *testing.T) {
	f := newPGFixture(t)
	if _, err := f.pool.Exec(f.ctx, `INSERT INTO audit_logs (action, entity_type) VALUES ('test.append','test')`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(f.ctx, `UPDATE audit_logs SET action='tampered' WHERE action='test.append'`); err == nil {
		t.Fatal("journal rows must not be updatable")
	}
	if _, err := f.pool.Exec(f.ctx, `DELETE FROM audit_logs WHERE action='test.append'`); err == nil {
		t.Fatal("journal rows must not be deletable")
	}
}
