// Command createuser provisions an account. There is no public registration, so this is the only way to create the first administrator.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/term"

	"github.com/ic-group/ic-tender/apps/api/internal/auth"
	"github.com/ic-group/ic-tender/apps/api/internal/config"
	"github.com/ic-group/ic-tender/apps/api/internal/db"
)

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+$`)

func main() {
	email := flag.String("email", "", "user email")
	role := flag.String("role", "", "SUPER_ADMIN, JUNIOR_ADMIN or SUBCONTRACTOR")
	subcontractorID := flag.Int64("subcontractor-id", 0, "subcontractors.id, required for SUBCONTRACTOR only")
	flag.Parse()

	if err := run(*email, auth.Role(*role), *subcontractorID); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(rawEmail string, role auth.Role, subcontractorID int64) error {
	email := auth.NormalizeEmail(rawEmail)
	switch {
	case !emailPattern.MatchString(email):
		return errors.New("-email must be a valid address")
	case !role.Valid():
		return errors.New("-role must be SUPER_ADMIN, JUNIOR_ADMIN or SUBCONTRACTOR")
	case role == auth.RoleSubcontractor && subcontractorID <= 0:
		return errors.New("-subcontractor-id is required for SUBCONTRACTOR")
	case role != auth.RoleSubcontractor && subcontractorID != 0:
		return errors.New("-subcontractor-id is allowed for SUBCONTRACTOR only")
	}

	password, err := readPassword()
	if err != nil {
		return err
	}
	if err := auth.ValidatePassword(password, email); err != nil {
		return fmt.Errorf("password rejected: need %d-%d characters and not equal to the email", auth.MinPasswordRunes, auth.MaxPasswordRunes)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	var sub *int64
	if subcontractorID > 0 {
		sub = &subcontractorID
	}
	var id int64
	err = pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, role, subcontractor_id, must_change_password) VALUES ($1, $2, $3, $4, TRUE) RETURNING id`,
		email, hash, string(role), sub).Scan(&id)
	var pgErr *pgconn.PgError
	switch {
	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		return errors.New("a user with this email already exists")
	case errors.As(err, &pgErr) && pgErr.Code == "23503":
		return errors.New("subcontractor not found")
	case err != nil:
		return err
	}
	if err := auth.NewPGStore(pool).Audit(ctx, auth.AuditEntry{Action: "user.created", EntityType: "user", EntityID: &id,
		After: map[string]any{"role": string(role)}, Reason: "created with the createuser command"}); err != nil {
		slog.Warn("audit write failed", "error", err)
	}
	fmt.Printf("created user id=%d email=%s role=%s (password change is required at the first login)\n", id, email, role)
	return nil
}

// readPassword takes the password from CREATEUSER_PASSWORD, a hidden terminal prompt or the first line of piped stdin.
func readPassword() (string, error) {
	if v := os.Getenv("CREATEUSER_PASSWORD"); v != "" {
		return v, nil
	}
	fd := int(syscall.Stdin)
	if term.IsTerminal(fd) {
		fmt.Fprint(os.Stderr, "Password: ")
		first, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		fmt.Fprint(os.Stderr, "Repeat password: ")
		second, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		if string(first) != string(second) {
			return "", errors.New("passwords do not match")
		}
		return string(first), nil
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", errors.New("no password provided on stdin")
	}
	return strings.TrimRight(line, "\r\n"), nil
}
