package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters follow the OWASP minimum (19 MiB, 2 iterations, 1 lane).
const (
	argonTime      uint32 = 2
	argonMemoryKiB uint32 = 19 * 1024
	argonThreads   uint8  = 1
	argonKeyLen    uint32 = 32
	argonSaltLen          = 16

	MinPasswordRunes = 12
	MaxPasswordRunes = 128
)

var ErrWeakPassword = errors.New("weak password")

func ValidatePassword(password, email string) error {
	n := utf8.RuneCountInString(password)
	switch {
	case n < MinPasswordRunes:
		return fmt.Errorf("%w: shorter than %d characters", ErrWeakPassword, MinPasswordRunes)
	case n > MaxPasswordRunes:
		return fmt.Errorf("%w: longer than %d characters", ErrWeakPassword, MaxPasswordRunes)
	case email != "" && strings.EqualFold(password, email):
		return fmt.Errorf("%w: equals the email", ErrWeakPassword)
	}
	return nil
}

func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemoryKiB, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemoryKiB, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

var errMalformedHash = errors.New("malformed password hash")

// VerifyPassword compares in constant time; needsRehash is true when the stored hash uses weaker parameters.
func VerifyPassword(encoded, password string) (match, needsRehash bool, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return false, false, errMalformedHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, false, errMalformedHash
	}
	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, false, errMalformedHash
	}
	if memory == 0 || memory > 256*1024 || time == 0 || time > 10 || threads == 0 || threads > 16 {
		return false, false, errMalformedHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 8 {
		return false, false, errMalformedHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) < 16 {
		return false, false, errMalformedHash
	}
	got := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(want)))
	needsRehash = memory != argonMemoryKiB || time != argonTime || threads != argonThreads || uint32(len(want)) != argonKeyLen
	return subtle.ConstantTimeCompare(got, want) == 1, needsRehash, nil
}
