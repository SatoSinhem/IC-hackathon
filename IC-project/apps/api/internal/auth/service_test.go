package auth_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ic-group/ic-tender/apps/api/internal/auth"
	"github.com/ic-group/ic-tender/apps/api/internal/auth/authtest"
)

const goodPassword = "correct horse battery"

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

type fixture struct {
	svc   *auth.Service
	store *authtest.MemStore
	clock *fakeClock
	admin auth.User
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	hash, err := auth.HashPassword(goodPassword)
	if err != nil {
		t.Fatal(err)
	}
	store := authtest.NewMemStore()
	clock := &fakeClock{t: time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)}
	svc, err := auth.NewService(store, auth.Config{
		Secret: []byte(strings.Repeat("k", 32)), IdleTimeout: 2 * time.Hour, AbsoluteTimeout: 7 * 24 * time.Hour,
		Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)), Now: clock.now,
	})
	if err != nil {
		t.Fatal(err)
	}
	admin := store.AddUser(auth.User{Email: "admin@ic.kz", PasswordHash: hash, Role: auth.RoleSuperAdmin, Active: true})
	return &fixture{svc: svc, store: store, clock: clock, admin: admin}
}

func (f *fixture) login(t *testing.T, email, password string) auth.LoginResult {
	t.Helper()
	res, err := f.svc.Login(context.Background(), email, password, "10.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	return res
}

func TestLoginSuccessCreatesSessionAndAudit(t *testing.T) {
	f := newFixture(t)
	res := f.login(t, "  Admin@IC.kz ", goodPassword)
	if len(res.Token) != 43 || res.User.ID != f.admin.ID {
		t.Fatalf("unexpected result %+v", res)
	}
	if want := f.clock.now().Add(7 * 24 * time.Hour); !res.ExpiresAt.Equal(want) {
		t.Fatalf("expires %v, want %v", res.ExpiresAt, want)
	}
	p, err := f.svc.Authenticate(context.Background(), res.Token)
	if err != nil || p.UserID != f.admin.ID || p.Role != auth.RoleSuperAdmin {
		t.Fatalf("principal %+v err %v", p, err)
	}
	if e := f.store.AuditEntries(); len(e) != 1 || e[0].Action != "auth.login.success" {
		t.Fatalf("audit = %+v", e)
	}
}

func TestLoginTokensAreUnique(t *testing.T) {
	f := newFixture(t)
	if a, b := f.login(t, "admin@ic.kz", goodPassword), f.login(t, "admin@ic.kz", goodPassword); a.Token == b.Token {
		t.Fatal("every login must get a fresh token")
	}
}

func TestLoginFailuresAreIndistinguishable(t *testing.T) {
	f := newFixture(t)
	hash, _ := auth.HashPassword("another password!")
	f.store.AddUser(auth.User{Email: "off@ic.kz", PasswordHash: hash, Role: auth.RoleJuniorAdmin, Active: false})

	cases := map[string][2]string{
		"unknown email":                     {"nobody@ic.kz", goodPassword},
		"wrong password":                    {"admin@ic.kz", "wrong password value"},
		"inactive user with right password": {"off@ic.kz", "another password!"},
		"empty password":                    {"admin@ic.kz", ""},
		"empty email":                       {"", goodPassword},
	}
	for name, c := range cases {
		_, err := f.svc.Login(context.Background(), c[0], c[1], "10.0.0.9", "")
		if !errors.Is(err, auth.ErrInvalidCredentials) {
			t.Errorf("%s: err = %v, want ErrInvalidCredentials", name, err)
		}
	}
	for _, e := range f.store.AuditEntries() {
		if e.Action == "auth.login.success" {
			t.Fatal("no failed attempt may be recorded as a success")
		}
		for _, v := range e.After {
			if s, ok := v.(string); ok && strings.Contains(s, "@") {
				t.Fatalf("attempted email leaked into the audit journal: %+v", e)
			}
		}
	}
}

func TestEmailLockoutAfterFiveFailuresEvenWithCorrectPassword(t *testing.T) {
	f := newFixture(t)
	for i := 0; i < 5; i++ {
		_, _ = f.svc.Login(context.Background(), "admin@ic.kz", "wrong password value", "10.0.0.2", "")
	}
	_, err := f.svc.Login(context.Background(), "admin@ic.kz", goodPassword, "10.0.0.2", "")
	var rl *auth.RateLimitedError
	if !errors.As(err, &rl) || rl.RetryAfter <= 0 || rl.RetryAfter > 15*time.Minute {
		t.Fatalf("expected RateLimitedError within 15 minutes, got %v", err)
	}
	// Another IP does not help: the lock is per account too.
	if _, err := f.svc.Login(context.Background(), "admin@ic.kz", goodPassword, "10.0.0.3", ""); !errors.As(err, &rl) {
		t.Fatalf("lock must apply from any address, got %v", err)
	}
	f.clock.advance(15*time.Minute + time.Second)
	f.login(t, "admin@ic.kz", goodPassword)
}

func TestIPLockoutAcrossManyAccounts(t *testing.T) {
	f := newFixture(t)
	for i := 0; i < 20; i++ {
		_, _ = f.svc.Login(context.Background(), strings.Repeat("a", i+1)+"@x.kz", "whatever password", "10.0.0.5", "")
	}
	_, err := f.svc.Login(context.Background(), "admin@ic.kz", goodPassword, "10.0.0.5", "")
	var rl *auth.RateLimitedError
	if !errors.As(err, &rl) {
		t.Fatalf("password spraying from one address must be limited, got %v", err)
	}
	f.login(t, "admin@ic.kz", goodPassword) // the same account from a different address still works
}

func TestSuccessfulLoginResetsAccountCounter(t *testing.T) {
	f := newFixture(t)
	for round := 0; round < 3; round++ {
		for i := 0; i < 4; i++ {
			_, _ = f.svc.Login(context.Background(), "admin@ic.kz", "wrong password value", "10.1.0.1", "")
		}
		f.login(t, "admin@ic.kz", goodPassword)
	}
}

func TestSessionExpiry(t *testing.T) {
	f := newFixture(t)
	res := f.login(t, "admin@ic.kz", goodPassword)

	f.clock.advance(119 * time.Minute)
	if _, err := f.svc.Authenticate(context.Background(), res.Token); err != nil {
		t.Fatalf("session must survive 119 minutes: %v", err)
	}
	// Activity refreshed the idle timer, so another 119 minutes are fine.
	f.clock.advance(119 * time.Minute)
	if _, err := f.svc.Authenticate(context.Background(), res.Token); err != nil {
		t.Fatalf("activity must extend the idle window: %v", err)
	}
	f.clock.advance(121 * time.Minute)
	if _, err := f.svc.Authenticate(context.Background(), res.Token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("idle session must expire, got %v", err)
	}

	// The absolute limit applies even with continuous activity.
	res = f.login(t, "admin@ic.kz", goodPassword)
	for i := 0; i < 7*24; i++ {
		f.clock.advance(time.Hour)
		if i < 7*24-1 {
			if _, err := f.svc.Authenticate(context.Background(), res.Token); err != nil {
				t.Fatalf("hour %d: %v", i, err)
			}
		}
	}
	if _, err := f.svc.Authenticate(context.Background(), res.Token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("session must end after 7 days, got %v", err)
	}
}

func TestLogoutRevokesSessionAndIsAudited(t *testing.T) {
	f := newFixture(t)
	res := f.login(t, "admin@ic.kz", goodPassword)
	if err := f.svc.Logout(context.Background(), res.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Authenticate(context.Background(), res.Token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("revoked session must not authenticate, got %v", err)
	}
	found := false
	for _, e := range f.store.AuditEntries() {
		found = found || e.Action == "auth.logout"
	}
	if !found {
		t.Fatal("logout must be audited")
	}
	if err := f.svc.Logout(context.Background(), "garbage"); err != nil {
		t.Fatalf("logout with a bogus token must be a no-op: %v", err)
	}
}

func TestAuthenticateRejectsMalformedAndForgedTokens(t *testing.T) {
	f := newFixture(t)
	f.login(t, "admin@ic.kz", goodPassword)
	for _, tok := range []string{"", "short", strings.Repeat("A", 43), strings.Repeat("A", 44), strings.Repeat("!", 43), "../../etc/passwd"} {
		if _, err := f.svc.Authenticate(context.Background(), tok); !errors.Is(err, auth.ErrUnauthenticated) {
			t.Errorf("token %q: err = %v", tok, err)
		}
	}
}

func TestDeactivatedUserLosesExistingSessions(t *testing.T) {
	f := newFixture(t)
	res := f.login(t, "admin@ic.kz", goodPassword)
	f.store.SetActive(f.admin.ID, false)
	if _, err := f.svc.Authenticate(context.Background(), res.Token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("suspended user must be signed out immediately, got %v", err)
	}
}

func TestSessionTokenHashDependsOnSecret(t *testing.T) {
	f := newFixture(t)
	res := f.login(t, "admin@ic.kz", goodPassword)
	other, _ := auth.NewService(f.store, auth.Config{Secret: []byte(strings.Repeat("z", 32)), Now: f.clock.now, Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))})
	if _, err := other.Authenticate(context.Background(), res.Token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("a different secret must invalidate sessions, got %v", err)
	}
}

func TestChangePassword(t *testing.T) {
	f := newFixture(t)
	first := f.login(t, "admin@ic.kz", goodPassword)
	second := f.login(t, "admin@ic.kz", goodPassword)
	p, _ := f.svc.Authenticate(context.Background(), first.Token)

	if err := f.svc.ChangePassword(context.Background(), p, "not the password", "brand new passphrase", "10.0.0.1"); !errors.Is(err, auth.ErrWrongCurrentPassword) {
		t.Fatalf("wrong current password: %v", err)
	}
	for _, weak := range []string{"short", goodPassword, "admin@ic.kz"} {
		if err := f.svc.ChangePassword(context.Background(), p, goodPassword, weak, "10.0.0.1"); !errors.Is(err, auth.ErrWeakPassword) {
			t.Fatalf("new password %q must be rejected, got %v", weak, err)
		}
	}
	if err := f.svc.ChangePassword(context.Background(), p, goodPassword, "brand new passphrase", "10.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Authenticate(context.Background(), first.Token); err != nil {
		t.Fatalf("the session that changed the password must stay valid: %v", err)
	}
	if _, err := f.svc.Authenticate(context.Background(), second.Token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("other sessions must be revoked, got %v", err)
	}
	if _, err := f.svc.Login(context.Background(), "admin@ic.kz", goodPassword, "10.0.0.1", ""); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("old password must stop working, got %v", err)
	}
	f.login(t, "admin@ic.kz", "brand new passphrase")
}

func TestChangePasswordClearsMustChangeAndIsRateLimited(t *testing.T) {
	f := newFixture(t)
	hash, _ := auth.HashPassword("initial password")
	u := f.store.AddUser(auth.User{Email: "new@ic.kz", PasswordHash: hash, Role: auth.RoleJuniorAdmin, Active: true, MustChangePassword: true})
	res := f.login(t, "new@ic.kz", "initial password")
	p, _ := f.svc.Authenticate(context.Background(), res.Token)
	if !p.MustChangePassword {
		t.Fatal("principal must carry the must-change flag")
	}
	for i := 0; i < 5; i++ {
		_ = f.svc.ChangePassword(context.Background(), p, "guess guess guess", "whatever new passphrase", "10.0.0.1")
	}
	var rl *auth.RateLimitedError
	if err := f.svc.ChangePassword(context.Background(), p, "initial password", "whatever new passphrase", "10.0.0.1"); !errors.As(err, &rl) {
		t.Fatalf("current-password guessing must be limited, got %v", err)
	}
	f.clock.advance(16 * time.Minute)
	if err := f.svc.ChangePassword(context.Background(), p, "initial password", "whatever new passphrase", "10.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if f.store.User(u.ID).MustChangePassword {
		t.Fatal("changing the password must clear the flag")
	}
}

func TestAuditFailureDoesNotBlockLogin(t *testing.T) {
	f := newFixture(t)
	f.store.FailAudit(true)
	f.login(t, "admin@ic.kz", goodPassword)
}

func TestPrincipalPermissions(t *testing.T) {
	sup := auth.Principal{Role: auth.RoleSuperAdmin}
	jun := auth.Principal{Role: auth.RoleJuniorAdmin, Permissions: map[string]bool{"AWARD_WINNER": true}}
	bare := auth.Principal{Role: auth.RoleJuniorAdmin}
	sub := auth.Principal{Role: auth.RoleSubcontractor, Permissions: map[string]bool{"AWARD_WINNER": true}}
	if !sup.Can("ANYTHING") || !jun.Can("AWARD_WINNER") || jun.Can("OTHER") || bare.Can("AWARD_WINNER") || sub.Can("AWARD_WINNER") {
		t.Fatal("permission rules are wrong")
	}
	if sub.IsAdmin() || !jun.IsAdmin() || !sup.HasRole(auth.RoleSuperAdmin, auth.RoleJuniorAdmin) || sub.HasRole(auth.RoleSuperAdmin) {
		t.Fatal("role helpers are wrong")
	}
}
