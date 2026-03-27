package app

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPAddr              = ":8080"
	defaultMetricsAddr           = ":9090"
	defaultWebhookReadTimeout    = 5 * time.Second
	defaultWebhookWriteTimeout   = 5 * time.Second
	defaultMaxWebhookBodyBytes   = 256 * 1024
	defaultWorkerConcurrency     = 8
	defaultWorkerLeaseSeconds    = 60
	defaultWorkerMaxRetry        = 8
	defaultWorkerRateLimitRPS    = 20
	defaultRetentionJobsDays     = 30
	defaultRetentionDedupeDays   = 14
	defaultRetentionPayloadHours = 24
)

type Config struct {
	Env      string
	LogLevel string

	HTTPAddr    string
	MetricsAddr string

	DBDSN string

	InviteHashPepper string

	TelegramToken         string
	TelegramWebhookSecret string

	MaxToken         string
	MaxWebhookSecret string
	MaxAPIBaseURL    string

	WebhookReadTimeout  time.Duration
	WebhookWriteTimeout time.Duration
	MaxWebhookBodyBytes int64

	WorkerConcurrency  int
	WorkerLease        time.Duration
	WorkerMaxRetry     int
	WorkerRateLimitRPS int

	RetentionJobsDays     int
	RetentionDedupeDays   int
	RetentionPayloadHours int
}

func LoadConfig() (Config, error) {
	cfg := Config{
		Env:                 getenv("APP_ENV", "dev"),
		LogLevel:            getenv("LOG_LEVEL", "info"),
		HTTPAddr:            getenv("HTTP_ADDR", defaultHTTPAddr),
		MetricsAddr:         getenv("METRICS_ADDR", defaultMetricsAddr),
		MaxAPIBaseURL:       getenv("MAX_API_BASE_URL", "https://botapi.max.ru"),
		WebhookReadTimeout:  parseDuration("WEBHOOK_READ_TIMEOUT", defaultWebhookReadTimeout),
		WebhookWriteTimeout: parseDuration("WEBHOOK_WRITE_TIMEOUT", defaultWebhookWriteTimeout),
		MaxWebhookBodyBytes: parseInt64("MAX_WEBHOOK_BODY_BYTES", defaultMaxWebhookBodyBytes),
		WorkerConcurrency:   parseInt("WORKER_CONCURRENCY", defaultWorkerConcurrency),
		WorkerLease:         time.Duration(parseInt("WORKER_LEASE_SECONDS", defaultWorkerLeaseSeconds)) * time.Second,
		WorkerMaxRetry:      parseInt("WORKER_MAX_RETRY", defaultWorkerMaxRetry),
		WorkerRateLimitRPS:  parseInt("WORKER_RATE_LIMIT_RPS", defaultWorkerRateLimitRPS),
		RetentionJobsDays:   parseInt("RETENTION_JOBS_DAYS", defaultRetentionJobsDays),
		RetentionDedupeDays: parseInt("RETENTION_DEDUPE_DAYS", defaultRetentionDedupeDays),
		RetentionPayloadHours: parseInt(
			"RETENTION_PAYLOAD_HOURS",
			defaultRetentionPayloadHours,
		),
	}

	cfg.DBDSN = readSecret("DB_DSN", "DB_DSN_FILE")
	cfg.InviteHashPepper = readSecret("INVITE_HASH_PEPPER", "INVITE_HASH_PEPPER_FILE")
	cfg.TelegramToken = readSecret("TELEGRAM_BOT_TOKEN", "TELEGRAM_BOT_TOKEN_FILE")
	cfg.TelegramWebhookSecret = readSecret("TELEGRAM_WEBHOOK_SECRET", "TELEGRAM_WEBHOOK_SECRET_FILE")
	cfg.MaxToken = readSecret("MAX_BOT_TOKEN", "MAX_BOT_TOKEN_FILE")
	cfg.MaxWebhookSecret = readSecret("MAX_WEBHOOK_SECRET", "MAX_WEBHOOK_SECRET_FILE")

	if cfg.DBDSN == "" {
		return Config{}, errors.New("missing DB_DSN or DB_DSN_FILE")
	}
	if cfg.InviteHashPepper == "" {
		return Config{}, errors.New("missing INVITE_HASH_PEPPER or INVITE_HASH_PEPPER_FILE")
	}
	if cfg.TelegramWebhookSecret == "" {
		return Config{}, errors.New("missing TELEGRAM_WEBHOOK_SECRET or TELEGRAM_WEBHOOK_SECRET_FILE")
	}
	if cfg.TelegramToken == "" {
		return Config{}, errors.New("missing TELEGRAM_BOT_TOKEN or TELEGRAM_BOT_TOKEN_FILE")
	}
	if cfg.MaxWebhookSecret == "" {
		return Config{}, errors.New("missing MAX_WEBHOOK_SECRET or MAX_WEBHOOK_SECRET_FILE")
	}
	if cfg.MaxToken == "" {
		return Config{}, errors.New("missing MAX_BOT_TOKEN or MAX_BOT_TOKEN_FILE")
	}

	return cfg, nil
}

func readSecret(valueKey, fileKey string) string {
	if val := os.Getenv(valueKey); val != "" {
		return val
	}
	path := os.Getenv(fileKey)
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseInt(key string, fallback int) int {
	if raw := os.Getenv(key); raw != "" {
		val, err := strconv.Atoi(raw)
		if err == nil {
			return val
		}
	}
	return fallback
}

func parseInt64(key string, fallback int64) int64 {
	if raw := os.Getenv(key); raw != "" {
		val, err := strconv.ParseInt(raw, 10, 64)
		if err == nil {
			return val
		}
	}
	return fallback
}

func parseDuration(key string, fallback time.Duration) time.Duration {
	if raw := os.Getenv(key); raw != "" {
		val, err := time.ParseDuration(raw)
		if err == nil {
			return val
		}
	}
	return fallback
}
