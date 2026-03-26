package app

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

func NewLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl, ReplaceAttr: redactAttr})
	return slog.New(h)
}

func redactAttr(_ []string, a slog.Attr) slog.Attr {
	key := strings.ToLower(a.Key)
	if strings.Contains(key, "token") || strings.Contains(key, "secret") || strings.Contains(key, "password") || strings.Contains(key, "invite") {
		return slog.String(a.Key, "[redacted]")
	}
	return a
}

type loggerKey struct{}

func WithLogger(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, log)
}

func LoggerFrom(ctx context.Context) *slog.Logger {
	v := ctx.Value(loggerKey{})
	if v == nil {
		return slog.Default()
	}
	l, ok := v.(*slog.Logger)
	if !ok {
		return slog.Default()
	}
	return l
}
