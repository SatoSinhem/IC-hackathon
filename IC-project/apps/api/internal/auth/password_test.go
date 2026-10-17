package auth_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ic-group/ic-tender/apps/api/internal/auth"
)

func TestHashAndVerifyRoundTrip(t *testing.T) {
	hash, err := auth.HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("unexpected format %q", hash)
	}
	ok, rehash, err := auth.VerifyPassword(hash, "correct horse battery")
	if err != nil || !ok || rehash {
		t.Fatalf("ok=%v rehash=%v err=%v", ok, rehash, err)
	}
	if ok, _, _ := auth.VerifyPassword(hash, "correct horse batterY"); ok {
		t.Fatal("a different password must not verify")
	}
}

func TestHashesAreSalted(t *testing.T) {
	a, _ := auth.HashPassword("same password value")
	b, _ := auth.HashPassword("same password value")
	if a == b {
		t.Fatal("two hashes of one password must differ")
	}
}

func TestVerifyRejectsMalformedHashes(t *testing.T) {
	for _, h := range []string{
		"", "plaintext", "$argon2id$v=19$m=1,t=1$salt$hash",
		"$bcrypt$v=19$m=19456,t=2,p=1$c2FsdHNhbHQ$aGFzaGhhc2hoYXNoaGFzaA",
		"$argon2id$v=19$m=999999999,t=2,p=1$c2FsdHNhbHRzYWx0$aGFzaGhhc2hoYXNoaGFzaGhhc2g",
		"$argon2id$v=19$m=19456,t=99,p=1$c2FsdHNhbHRzYWx0$aGFzaGhhc2hoYXNoaGFzaGhhc2g",
	} {
		if ok, _, err := auth.VerifyPassword(h, "x"); ok || err == nil {
			t.Fatalf("hash %q must be rejected, ok=%v err=%v", h, ok, err)
		}
	}
}

func TestVerifyFlagsWeakerParametersForRehash(t *testing.T) {
	// Produced with m=8192,t=1,p=1 for the password "legacy-password".
	weak := "$argon2id$v=19$m=8192,t=1,p=1$c29tZXNhbHRzb21lc2FsdA$" + strings.Repeat("A", 43)
	_, rehash, err := auth.VerifyPassword(weak, "legacy-password")
	if err != nil || !rehash {
		t.Fatalf("rehash=%v err=%v", rehash, err)
	}
}

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		name, password, email string
		wantErr               bool
	}{
		{"too short", "short", "a@b.kz", true},
		{"eleven chars", "12345678901", "a@b.kz", true},
		{"twelve chars", "123456789012", "a@b.kz", false},
		{"too long", strings.Repeat("a", 129), "a@b.kz", true},
		{"max length", strings.Repeat("a", 128), "a@b.kz", false},
		{"equals email", "longer.name@example.com", "Longer.Name@example.com", true},
		{"cyrillic counts runes", "парольпароль", "a@b.kz", false},
		{"spaces are allowed", "a long passphrase", "a@b.kz", false},
	}
	for _, tc := range cases {
		err := auth.ValidatePassword(tc.password, tc.email)
		if (err != nil) != tc.wantErr || (err != nil && !errors.Is(err, auth.ErrWeakPassword)) {
			t.Errorf("%s: err = %v, wantErr = %v", tc.name, err, tc.wantErr)
		}
	}
}
