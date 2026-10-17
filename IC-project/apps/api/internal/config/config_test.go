package config

import (
	"strings"
	"testing"
)

func devEnv() map[string]string {
	return map[string]string{"DATABASE_URL": "postgres://u:p@localhost:5432/db?sslmode=disable"}
}

func prodEnv() map[string]string {
	return map[string]string{
		"APP_ENV":             "production",
		"DATABASE_URL":        "postgres://u:p@db.example:5432/db?sslmode=require",
		"WEB_ORIGIN":          "https://tender.example.com",
		"SESSION_SECRET":      strings.Repeat("a", 32),
		"INTERNAL_JOB_SECRET": strings.Repeat("b", 32),
	}
}

func loadFrom(m map[string]string) (Config, error) {
	return load(func(key string) string { return m[key] })
}

func TestLoadDevelopmentDefaults(t *testing.T) {
	cfg, err := loadFrom(devEnv())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Env != "development" || cfg.APIAddr != ":8080" || cfg.WebOrigin != "http://localhost:3000" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.IsProduction() {
		t.Fatal("development must not report production")
	}
}

func TestLoadTrimsTrailingSlashOfOrigin(t *testing.T) {
	m := devEnv()
	m["WEB_ORIGIN"] = "http://localhost:3000/"
	cfg, err := loadFrom(m)
	if err != nil || cfg.WebOrigin != "http://localhost:3000" {
		t.Fatalf("origin = %q, err = %v", cfg.WebOrigin, err)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	_, err := loadFrom(map[string]string{})
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL is required") {
		t.Fatalf("expected DATABASE_URL error, got %v", err)
	}
}

func TestLoadTrustForwardedFor(t *testing.T) {
	cases := map[string]struct {
		value   string
		want    bool
		wantErr bool
	}{
		"unset is false":  {"", false, false},
		"false":           {"false", false, false},
		"zero":            {"0", false, false},
		"true":            {"true", true, false},
		"upper case":      {"TRUE", true, false},
		"one":             {"1", true, false},
		"typo is refused": {"yes", false, true},
	}
	for name, tc := range cases {
		m := devEnv()
		m["TRUST_FORWARDED_FOR"] = tc.value
		cfg, err := loadFrom(m)
		if (err != nil) != tc.wantErr || (err == nil && cfg.TrustForwardedFor != tc.want) {
			t.Errorf("%s: got %v, err %v", name, cfg.TrustForwardedFor, err)
		}
	}
}

func TestLoadErrorsNeverLeakSecrets(t *testing.T) {
	m := devEnv()
	m["DATABASE_URL"] = "mysql://u:secret-pass@host/db"
	m["SESSION_SECRET"] = "short-secret-value"
	_, err := loadFrom(m)
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, leaked := range []string{"secret-pass", "short-secret-value"} {
		if strings.Contains(err.Error(), leaked) {
			t.Fatalf("error leaks %q: %v", leaked, err)
		}
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(map[string]string)
		want   string
	}{
		{"unknown env", func(m map[string]string) { m["APP_ENV"] = "staging" }, "APP_ENV"},
		{"non-postgres url", func(m map[string]string) { m["DATABASE_URL"] = "mysql://u:p@h/db" }, "postgres://"},
		{"origin with path", func(m map[string]string) { m["WEB_ORIGIN"] = "http://localhost:3000/app" }, "WEB_ORIGIN"},
		{"origin without scheme", func(m map[string]string) { m["WEB_ORIGIN"] = "localhost:3000" }, "WEB_ORIGIN"},
		{"short session secret", func(m map[string]string) { m["SESSION_SECRET"] = "short" }, "SESSION_SECRET must be at least"},
		{"same secrets", func(m map[string]string) {
			m["SESSION_SECRET"] = strings.Repeat("a", 32)
			m["INTERNAL_JOB_SECRET"] = strings.Repeat("a", 32)
		}, "must differ"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := devEnv()
			tc.mutate(m)
			if _, err := loadFrom(m); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestLoadProductionValid(t *testing.T) {
	cfg, err := loadFrom(prodEnv())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.IsProduction() {
		t.Fatal("expected production")
	}
}

func TestLoadProductionRejectsUnsafeSettings(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(map[string]string)
		want   string
	}{
		{"sslmode disable", func(m map[string]string) { m["DATABASE_URL"] = "postgres://u:p@h:5432/db?sslmode=disable" }, "sslmode"},
		{"sslmode missing", func(m map[string]string) { m["DATABASE_URL"] = "postgres://u:p@h:5432/db" }, "sslmode"},
		{"http origin", func(m map[string]string) { m["WEB_ORIGIN"] = "http://tender.example.com" }, "https"},
		{"origin missing", func(m map[string]string) { delete(m, "WEB_ORIGIN") }, "WEB_ORIGIN is required"},
		{"session secret missing", func(m map[string]string) { delete(m, "SESSION_SECRET") }, "SESSION_SECRET is required"},
		{"job secret missing", func(m map[string]string) { delete(m, "INTERNAL_JOB_SECRET") }, "INTERNAL_JOB_SECRET is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := prodEnv()
			tc.mutate(m)
			if _, err := loadFrom(m); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}
