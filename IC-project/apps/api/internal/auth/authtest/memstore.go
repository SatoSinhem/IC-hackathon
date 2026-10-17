// Package authtest provides an in-memory auth.Store with the same session semantics as the PostgreSQL store.
package authtest

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/ic-group/ic-tender/apps/api/internal/auth"
)

type session struct {
	id        int64
	userID    int64
	hash      []byte
	createdAt time.Time
	expiresAt time.Time
	lastSeen  time.Time
	revoked   bool
}

type MemStore struct {
	mu          sync.Mutex
	nextID      int64
	users       map[int64]auth.User
	permissions map[int64][]string
	sessions    []*session
	audit       []auth.AuditEntry
	failAudit   bool
}

func NewMemStore() *MemStore {
	return &MemStore{users: map[int64]auth.User{}, permissions: map[int64][]string{}}
}

func (m *MemStore) AddUser(u auth.User, permissions ...string) auth.User {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	u.ID = m.nextID
	m.users[u.ID] = u
	m.permissions[u.ID] = permissions
	return u
}

func (m *MemStore) SetActive(userID int64, active bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.users[userID]
	u.Active = active
	m.users[userID] = u
}

func (m *MemStore) FailAudit(fail bool) { m.mu.Lock(); m.failAudit = fail; m.mu.Unlock() }

func (m *MemStore) AuditEntries() []auth.AuditEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]auth.AuditEntry(nil), m.audit...)
}

func (m *MemStore) ActiveSessions(userID int64) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, s := range m.sessions {
		if s.userID == userID && !s.revoked {
			n++
		}
	}
	return n
}

func (m *MemStore) User(id int64) auth.User {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.users[id]
}

func (m *MemStore) UserByEmail(_ context.Context, email string) (auth.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return auth.User{}, auth.ErrNotFound
}

func (m *MemStore) UserByID(_ context.Context, id int64) (auth.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return auth.User{}, auth.ErrNotFound
}

func (m *MemStore) CreateSession(_ context.Context, n auth.NewSession) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	m.sessions = append(m.sessions, &session{id: m.nextID, userID: n.UserID, hash: n.TokenHash, createdAt: n.CreatedAt, expiresAt: n.ExpiresAt, lastSeen: n.CreatedAt})
	return m.nextID, nil
}

func (m *MemStore) PrincipalBySession(_ context.Context, hash []byte, now, idleCutoff time.Time) (auth.Principal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		if !bytes.Equal(s.hash, hash) {
			continue
		}
		u := m.users[s.userID]
		if s.revoked || !s.expiresAt.After(now) || !s.lastSeen.After(idleCutoff) || !u.Active {
			return auth.Principal{}, auth.ErrNotFound
		}
		s.lastSeen = now
		perms := map[string]bool{}
		for _, p := range m.permissions[u.ID] {
			perms[p] = true
		}
		return auth.Principal{UserID: u.ID, Email: u.Email, Role: u.Role, SubcontractorID: u.SubcontractorID, SessionID: s.id, MustChangePassword: u.MustChangePassword, Permissions: perms}, nil
	}
	return auth.Principal{}, auth.ErrNotFound
}

func (m *MemStore) RevokeSession(_ context.Context, hash []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		if bytes.Equal(s.hash, hash) {
			s.revoked = true
		}
	}
	return nil
}

func (m *MemStore) RevokeOtherSessions(_ context.Context, userID, keep int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		if s.userID == userID && s.id != keep {
			s.revoked = true
		}
	}
	return nil
}

func (m *MemStore) SetPassword(_ context.Context, userID int64, hash string, clearMustChange bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.users[userID]
	u.PasswordHash = hash
	if clearMustChange {
		u.MustChangePassword = false
	}
	m.users[userID] = u
	return nil
}

func (m *MemStore) TouchLogin(context.Context, int64, time.Time) error { return nil }

func (m *MemStore) Audit(_ context.Context, e auth.AuditEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failAudit {
		return context.DeadlineExceeded
	}
	m.audit = append(m.audit, e)
	return nil
}
