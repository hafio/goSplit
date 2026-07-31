package config

import (
	"strings"
	"testing"
	"time"
)

func TestLooksLikePostgresKeywordDSN(t *testing.T) {
	yes := []string{
		"host=db port=5432 user=u password=p dbname=app",
		"host='db-postgre' port='5432' user='splitpro' password='xxfIh]UG^Gf3[.=^ALwn' dbname='splitpro'",
		"dbname=app",
		"user=u password=has=equals=inside dbname=app",
	}
	for _, s := range yes {
		if !looksLikePostgresKeywordDSN(s) {
			t.Errorf("expected keyword DSN: %q", s)
		}
	}
	no := []string{
		"file:./data/gosplit.db",
		"postgres://u:p@host:5432/db", // URL form is matched by the scheme check, not this
		"sqlite:///tmp/x.db",
		"./data/gosplit.db",
		"",
	}
	for _, s := range no {
		if looksLikePostgresKeywordDSN(s) {
			t.Errorf("did not expect keyword DSN: %q", s)
		}
	}
}

// validSecret is a 32-char session secret that satisfies Load's length floor.
const validSecret = "0123456789abcdef0123456789abcdef"

func TestLoadRequiresSessionSecret(t *testing.T) {
	t.Setenv("SESSION_SECRET", "")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "SESSION_SECRET is required") {
		t.Fatalf("Load without SESSION_SECRET err = %v, want 'required'", err)
	}
}

func TestLoadRejectsShortSecret(t *testing.T) {
	t.Setenv("SESSION_SECRET", "too-short") // 9 chars < 16
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "too short") {
		t.Fatalf("Load with a short secret err = %v, want 'too short'", err)
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("SESSION_SECRET", validSecret)
	// Force the defaulted fields off the ambient environment so we assert defaults.
	t.Setenv("BASE_URL", "")
	t.Setenv("ADDR", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("FROM_EMAIL", "")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.BaseURL != "http://localhost:8080" {
		t.Errorf("BaseURL = %q", c.BaseURL)
	}
	if c.Addr != ":8080" {
		t.Errorf("Addr = %q", c.Addr)
	}
	if c.DatabaseURL != "file:./data/gosplit.db" || c.Engine != EngineSQLite {
		t.Errorf("DatabaseURL/Engine = %q/%q", c.DatabaseURL, c.Engine)
	}
	if c.LogLevel != "info" || c.FromEmail != "no-reply@gosplit.local" {
		t.Errorf("LogLevel/FromEmail = %q/%q", c.LogLevel, c.FromEmail)
	}
	if c.SecureCookies {
		t.Error("SecureCookies should be false for an http:// BaseURL")
	}
}

func TestLoadEngineInference(t *testing.T) {
	cases := []struct {
		dsn     string
		want    Engine
		wantErr bool
	}{
		{"postgres://u:p@h:5432/db", EnginePostgres, false},
		{"postgresql://u:p@h:5432/db", EnginePostgres, false},
		{"host=db port=5432 dbname=app", EnginePostgres, false},
		{"file:./x.db", EngineSQLite, false},
		{"sqlite:///tmp/x.db", EngineSQLite, false},
		{"mysql://u:p@h/db", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.dsn, func(t *testing.T) {
			t.Setenv("SESSION_SECRET", validSecret)
			t.Setenv("DATABASE_URL", tc.dsn)
			c, err := Load()
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "cannot infer DB engine") {
					t.Fatalf("Load(%q) err = %v, want an infer error", tc.dsn, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load(%q): %v", tc.dsn, err)
			}
			if c.Engine != tc.want {
				t.Errorf("Load(%q).Engine = %q, want %q", tc.dsn, c.Engine, tc.want)
			}
		})
	}
}

func TestLoadSecureCookiesFromHTTPS(t *testing.T) {
	t.Setenv("SESSION_SECRET", validSecret)
	t.Setenv("BASE_URL", "https://gosplit.example")
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !c.SecureCookies {
		t.Error("SecureCookies should be true for an https:// BaseURL")
	}
}

func TestIsAdminEmail(t *testing.T) {
	c := &Config{AdminEmails: []string{"Admin@X.test", "  boss@y.test  "}}
	for _, e := range []string{"admin@x.test", "ADMIN@X.TEST", " boss@y.test "} {
		if !c.IsAdminEmail(e) {
			t.Errorf("IsAdminEmail(%q) = false, want true (case/space-insensitive)", e)
		}
	}
	if c.IsAdminEmail("nobody@z.test") {
		t.Error("IsAdminEmail for a non-listed address should be false")
	}
	if (&Config{}).IsAdminEmail("anyone@x.test") {
		t.Error("IsAdminEmail with an empty allowlist should be false")
	}
}

func TestGetEnv(t *testing.T) {
	t.Setenv("GS_TEST_STR", "value")
	if got := getEnv("GS_TEST_STR", "def"); got != "value" {
		t.Errorf("getEnv set = %q, want value", got)
	}
	t.Setenv("GS_TEST_STR", "")
	if got := getEnv("GS_TEST_STR", "def"); got != "def" {
		t.Errorf("getEnv empty = %q, want the default", got)
	}
}

func TestGetEnvInt(t *testing.T) {
	t.Setenv("GS_TEST_INT", "42")
	if got := getEnvInt("GS_TEST_INT", 7); got != 42 {
		t.Errorf("getEnvInt valid = %d, want 42", got)
	}
	t.Setenv("GS_TEST_INT", "not-a-number")
	if got := getEnvInt("GS_TEST_INT", 7); got != 7 {
		t.Errorf("getEnvInt invalid = %d, want the default 7", got)
	}
}

func TestGetEnvBool(t *testing.T) {
	t.Setenv("GS_TEST_BOOL", "true")
	if !getEnvBool("GS_TEST_BOOL", false) {
		t.Error("getEnvBool(true) = false")
	}
	t.Setenv("GS_TEST_BOOL", "nonsense")
	if !getEnvBool("GS_TEST_BOOL", true) {
		t.Error("getEnvBool(invalid) should fall back to the default true")
	}
}

func TestGetEnvList(t *testing.T) {
	t.Setenv("GS_TEST_LIST", "a, b ,,c")
	got := getEnvList("GS_TEST_LIST", nil)
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("getEnvList = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("getEnvList[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	t.Setenv("GS_TEST_LIST", "   ")
	if got := getEnvList("GS_TEST_LIST", []string{"d"}); len(got) != 1 || got[0] != "d" {
		t.Errorf("getEnvList(blank) = %v, want the default [d]", got)
	}
}

func TestGetEnvDuration(t *testing.T) {
	t.Setenv("GS_TEST_DUR", "90m")
	if got := getEnvDuration("GS_TEST_DUR", time.Hour); got != 90*time.Minute {
		t.Errorf("getEnvDuration valid = %v, want 90m", got)
	}
	t.Setenv("GS_TEST_DUR", "not-a-duration")
	if got := getEnvDuration("GS_TEST_DUR", time.Hour); got != time.Hour {
		t.Errorf("getEnvDuration invalid = %v, want the default 1h", got)
	}
}
