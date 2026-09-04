// Package config loads all runtime configuration from environment variables
// into a single typed struct. One source of config, matching self-host
// expectations. Parsing is plain os.Getenv; the sole external dependency is
// the cron parser, so that an invalid BACKUP_CRON fails at startup rather than
// at the first missed backup.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// Config is the fully-resolved application configuration.
type Config struct {
	// Core
	BaseURL       string
	Addr          string
	SessionSecret string
	LogLevel      string // debug | info (default) | warn | error

	// Database. DatabaseURL scheme selects the engine:
	//   file:./data/gosplit.db  or  sqlite://...  -> SQLite (default)
	//   postgres://...            -> PostgreSQL
	DatabaseURL string
	Engine      Engine

	// SMTP / email
	FromEmail                  string
	EmailServerHost            string
	EmailServerPort            int
	EmailServerUser            string
	EmailServerPassword        string
	EmailTLSRejectUnauthorized bool

	// Web push (VAPID)
	WebPushPublicKey  string
	WebPushPrivateKey string
	WebPushEmail      string

	// Plaid (optional, feature-flagged)
	PlaidClientID       string
	PlaidSecret         string
	PlaidEnvironment    string
	PlaidCountryCodes   []string
	PlaidIntervalInDays int

	// Currency rates
	CurrencyRateProvider   string
	OpenExchangeRatesAppID string

	// Uploads
	UploadMaxFileSizeMB int
	UploadDir           string

	// Scheduler
	Scheduler              bool
	ClearCacheCronRule     string
	CacheRetentionInterval time.Duration

	// Backup / restore. See internal/backup.
	BackupDir                string
	BackupCron               string // 5-field cron rule; "" disables scheduled backups
	BackupRetentionCount     int    // archives to keep in BackupDir; <= 0 keeps all
	AutoRestoreDir           string // watched at startup; "" disables auto-restore
	RestoreMaxUploadMB       int
	RestoreMaxArchiveBytes   int64
	RestoreMaxEntries        int
	RestoreMaxTableFileBytes int64
	RestoreMaxJSONLLineBytes int

	// AppVersion is the release tag stamped into the binary at link time. It is
	// assigned by main from its own `version` var, NOT read from the
	// environment, so that the tag the build was cut from is the only source.
	AppVersion string

	// Feature gates / misc
	DisableEmailSignup   bool
	EnableSendingInvites bool
	DefaultHomepage      string
	AdminEmails          []string
	FeedbackEmail        string
	DiscordWebhookURL    string

	// Cookie behaviour derived from BaseURL scheme.
	SecureCookies bool
}

// Engine identifies the SQL dialect in use.
type Engine string

const (
	EngineSQLite   Engine = "sqlite"
	EnginePostgres Engine = "postgres"
)

// Load reads configuration from the environment, applies defaults, and
// validates required values. It fails loud on systemic misconfiguration.
func Load() (*Config, error) {
	c := &Config{
		BaseURL:                    getEnv("BASE_URL", "http://localhost:8080"),
		Addr:                       getEnv("ADDR", ":8080"),
		SessionSecret:              os.Getenv("SESSION_SECRET"),
		DatabaseURL:                getEnv("DATABASE_URL", "file:./data/gosplit.db"),
		LogLevel:                   getEnv("LOG_LEVEL", "info"),
		FromEmail:                  getEnv("FROM_EMAIL", "no-reply@gosplit.local"),
		EmailServerHost:            os.Getenv("EMAIL_SERVER_HOST"),
		EmailServerPort:            getEnvInt("EMAIL_SERVER_PORT", 587),
		EmailServerUser:            os.Getenv("EMAIL_SERVER_USER"),
		EmailServerPassword:        os.Getenv("EMAIL_SERVER_PASSWORD"),
		EmailTLSRejectUnauthorized: getEnvBool("EMAIL_TLS_REJECT_UNAUTHORIZED", true),
		WebPushPublicKey:           os.Getenv("WEB_PUSH_PUBLIC_KEY"),
		WebPushPrivateKey:          os.Getenv("WEB_PUSH_PRIVATE_KEY"),
		WebPushEmail:               os.Getenv("WEB_PUSH_EMAIL"),
		PlaidClientID:              os.Getenv("PLAID_CLIENT_ID"),
		PlaidSecret:                os.Getenv("PLAID_SECRET"),
		PlaidEnvironment:           getEnv("PLAID_ENVIRONMENT", "sandbox"),
		PlaidCountryCodes:          getEnvList("PLAID_COUNTRY_CODES", []string{"US"}),
		PlaidIntervalInDays:        getEnvInt("PLAID_INTERVAL_IN_DAYS", 30),
		CurrencyRateProvider:       getEnv("CURRENCY_RATE_PROVIDER", "frankfurter"),
		OpenExchangeRatesAppID:     os.Getenv("OPEN_EXCHANGE_RATES_APP_ID"),
		UploadMaxFileSizeMB:        getEnvInt("UPLOAD_MAX_FILE_SIZE_MB", 10),
		UploadDir:                  getEnv("UPLOAD_DIR", "./data/uploads"),
		Scheduler:                  getEnvBool("SCHEDULER", true),
		ClearCacheCronRule:         getEnv("CLEAR_CACHE_CRON_RULE", "0 3 * * *"),
		CacheRetentionInterval:     getEnvDuration("CACHE_RETENTION_INTERVAL", 30*24*time.Hour),
		BackupDir:                  getEnv("BACKUP_DIR", "./data/backups"),
		BackupCron:                 os.Getenv("BACKUP_CRON"),
		BackupRetentionCount:       getEnvInt("BACKUP_RETENTION_COUNT", 7),
		AutoRestoreDir:             os.Getenv("AUTO_RESTORE_DIR"),
		RestoreMaxUploadMB:         getEnvInt("RESTORE_MAX_UPLOAD_MB", 500),
		RestoreMaxArchiveBytes:     int64(getEnvInt("RESTORE_MAX_ARCHIVE_MB", 2048)) << 20,
		RestoreMaxEntries:          getEnvInt("RESTORE_MAX_ENTRIES", 200000),
		RestoreMaxTableFileBytes:   int64(getEnvInt("RESTORE_MAX_TABLE_FILE_MB", 512)) << 20,
		RestoreMaxJSONLLineBytes:   getEnvInt("RESTORE_MAX_JSONL_LINE_MB", 8) << 20,
		DisableEmailSignup:         getEnvBool("DISABLE_EMAIL_SIGNUP", false),
		EnableSendingInvites:       getEnvBool("ENABLE_SENDING_INVITES", true),
		DefaultHomepage:            getEnv("DEFAULT_HOMEPAGE", "/balances"),
		AdminEmails:                getEnvList("ADMIN_EMAILS", nil),
		FeedbackEmail:              os.Getenv("FEEDBACK_EMAIL"),
		DiscordWebhookURL:          os.Getenv("DISCORD_WEBHOOK_URL"),
	}

	// Derive engine from DATABASE_URL: a postgres:// URL, a libpq keyword/value
	// DSN (host=... dbname=...), or a SQLite path/URL.
	switch {
	case strings.HasPrefix(c.DatabaseURL, "postgres://"), strings.HasPrefix(c.DatabaseURL, "postgresql://"),
		looksLikePostgresKeywordDSN(c.DatabaseURL):
		c.Engine = EnginePostgres
	case strings.HasPrefix(c.DatabaseURL, "file:"), strings.HasPrefix(c.DatabaseURL, "sqlite://"), c.DatabaseURL == "":
		c.Engine = EngineSQLite
	default:
		return nil, fmt.Errorf("config: cannot infer DB engine from DATABASE_URL %q (want file:/sqlite://, postgres://, or a host=... dbname=... keyword DSN)", c.DatabaseURL)
	}

	c.SecureCookies = strings.HasPrefix(c.BaseURL, "https://")

	// A session secret is mandatory; refuse to boot with an insecure default.
	if c.SessionSecret == "" {
		return nil, fmt.Errorf("config: SESSION_SECRET is required (generate 32+ random bytes)")
	}
	if len(c.SessionSecret) < 16 {
		return nil, fmt.Errorf("config: SESSION_SECRET too short (need >= 16 chars)")
	}

	if err := c.validateBackup(); err != nil {
		return nil, err
	}

	return c, nil
}

// validateBackup fails loud on a backup/restore misconfiguration. Each of
// these would otherwise surface much later, as a scheduled backup that never
// fires or -- worse -- a boot that restores the wrong archive.
func (c *Config) validateBackup() error {
	if c.BackupCron != "" {
		if _, err := BackupSchedule(c.BackupCron); err != nil {
			return fmt.Errorf("config: BACKUP_CRON %q is not a valid 5-field cron rule (min hour dom mon dow): %w", c.BackupCron, err)
		}
		if c.BackupDir == "" {
			return fmt.Errorf("config: BACKUP_CRON is set but BACKUP_DIR is empty (nowhere to write the archive)")
		}
	}
	if c.BackupRetentionCount == 0 {
		return fmt.Errorf("config: BACKUP_RETENTION_COUNT must be >= 1 to keep a bounded number of archives, or negative to keep all")
	}
	// A scheduled backup landing in the watched directory would make the next
	// boot restore the instance's own most recent backup.
	if c.AutoRestoreDir != "" && c.BackupDir != "" && sameDir(c.BackupDir, c.AutoRestoreDir) {
		return fmt.Errorf("config: BACKUP_DIR and AUTO_RESTORE_DIR must differ (both are %q): the server would restore its own scheduled backup on the next boot", c.BackupDir)
	}
	for name, v := range map[string]int64{
		"RESTORE_MAX_UPLOAD_MB":     int64(c.RestoreMaxUploadMB),
		"RESTORE_MAX_ARCHIVE_MB":    c.RestoreMaxArchiveBytes,
		"RESTORE_MAX_ENTRIES":       int64(c.RestoreMaxEntries),
		"RESTORE_MAX_TABLE_FILE_MB": c.RestoreMaxTableFileBytes,
		"RESTORE_MAX_JSONL_LINE_MB": int64(c.RestoreMaxJSONLLineBytes),
	} {
		if v <= 0 {
			return fmt.Errorf("config: %s must be > 0 (a zero cap would reject every archive)", name)
		}
	}
	return nil
}

// backupCronParser accepts standard 5-field cron expressions, matching how
// expense recurrences are parsed (see service.cronParser).
var backupCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// BackupSchedule parses a BACKUP_CRON rule. Exported so the scheduler reuses
// the exact parser Load validated with, rather than a second configuration of
// the same fields.
func BackupSchedule(rule string) (cron.Schedule, error) { return backupCronParser.Parse(rule) }

// sameDir reports whether two configured paths name the same directory, after
// cleaning. It is a path comparison, not a filesystem one: neither directory
// need exist yet at config time.
func sameDir(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

// pgKeywords are libpq/pgx connection keywords. Their presence as a
// space-separated key=value token marks a keyword/value Postgres DSN.
var pgKeywords = map[string]bool{
	"host": true, "hostaddr": true, "port": true, "user": true, "password": true,
	"dbname": true, "sslmode": true, "connect_timeout": true, "application_name": true,
	"options": true, "sslrootcert": true, "sslcert": true, "sslkey": true, "target_session_attrs": true,
}

// looksLikePostgresKeywordDSN reports whether s is a libpq keyword/value DSN
// (e.g. "host=db port=5432 user=u password=... dbname=app"). This form needs no
// URL-encoding of special characters in the password, unlike a postgres:// URL.
func looksLikePostgresKeywordDSN(s string) bool {
	for _, field := range strings.Fields(s) {
		if k, _, ok := strings.Cut(field, "="); ok && pgKeywords[k] {
			return true
		}
	}
	return false
}

// IsAdminEmail reports whether email is in the ADMIN_EMAILS allowlist
// (case-insensitive). Used for admin auto-promotion (never auto-demote).
func (c *Config) IsAdminEmail(email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	for _, a := range c.AdminEmails {
		if strings.ToLower(strings.TrimSpace(a)) == email {
			return true
		}
	}
	return false
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func getEnvList(key string, def []string) []string {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return def
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getEnvDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
