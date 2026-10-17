package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

const minSecretLength = 32

type Config struct {
	Env               string
	APIAddr           string
	DatabaseURL       string
	WebOrigin         string
	SessionSecret     string
	InternalJobSecret string
	// TrustForwardedFor must be true only when every request reaches the API through a proxy that appends the client address.
	TrustForwardedFor bool
}

func (c Config) IsProduction() bool { return c.Env == "production" }

// Load reads the configuration from the environment. Error messages never contain values, because they may be secrets.
func Load() (Config, error) { return load(os.Getenv) }

func load(get func(string) string) (Config, error) {
	env := valueOr(get("APP_ENV"), "development")
	origin := get("WEB_ORIGIN")
	if origin == "" && env != "production" {
		origin = "http://localhost:3000"
	}
	cfg := Config{
		Env:               env,
		APIAddr:           valueOr(get("API_ADDR"), ":8080"),
		DatabaseURL:       get("DATABASE_URL"),
		WebOrigin:         strings.TrimRight(origin, "/"),
		SessionSecret:     get("SESSION_SECRET"),
		InternalJobSecret: get("INTERNAL_JOB_SECRET"),
	}
	trust, err := parseBool(get("TRUST_FORWARDED_FOR"))
	if err != nil {
		return cfg, err
	}
	cfg.TrustForwardedFor = trust
	return cfg, cfg.validate()
}

func parseBool(raw string) (bool, error) {
	switch strings.ToLower(raw) {
	case "", "false", "0":
		return false, nil
	case "true", "1":
		return true, nil
	}
	return false, errors.New("TRUST_FORWARDED_FOR must be true or false")
}

func (c Config) validate() error {
	var errs []error
	if c.Env != "development" && c.Env != "production" {
		errs = append(errs, errors.New("APP_ENV must be development or production"))
	}
	prod := c.IsProduction()
	errs = append(errs,
		validateDatabaseURL(c.DatabaseURL, prod),
		validateWebOrigin(c.WebOrigin, prod),
		validateSecret("SESSION_SECRET", c.SessionSecret, prod),
		validateSecret("INTERNAL_JOB_SECRET", c.InternalJobSecret, prod),
	)
	if c.SessionSecret != "" && c.SessionSecret == c.InternalJobSecret {
		errs = append(errs, errors.New("SESSION_SECRET and INTERNAL_JOB_SECRET must differ"))
	}
	return errors.Join(errs...)
}

func validateDatabaseURL(raw string, prod bool) error {
	if raw == "" {
		return errors.New("DATABASE_URL is required")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
		return errors.New("DATABASE_URL must be a postgres:// URL with a host")
	}
	if prod {
		switch u.Query().Get("sslmode") {
		case "require", "verify-ca", "verify-full":
		default:
			return errors.New("DATABASE_URL must set sslmode to require, verify-ca or verify-full in production")
		}
	}
	return nil
}

func validateWebOrigin(raw string, prod bool) error {
	if raw == "" {
		return errors.New("WEB_ORIGIN is required in production")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.Path != "" || u.RawQuery != "" || u.User != nil {
		return errors.New("WEB_ORIGIN must be an origin like https://host[:port] without path")
	}
	if prod && u.Scheme != "https" {
		return errors.New("WEB_ORIGIN must use https in production")
	}
	return nil
}

func validateSecret(name, value string, required bool) error {
	if value == "" {
		if required {
			return fmt.Errorf("%s is required in production", name)
		}
		return nil
	}
	if len(value) < minSecretLength {
		return fmt.Errorf("%s must be at least %d characters", name, minSecretLength)
	}
	return nil
}

func valueOr(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
