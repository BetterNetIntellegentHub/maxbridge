package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"maxbridge/internal/app"
	"maxbridge/internal/domain"
	"maxbridge/internal/invites"
	maxapi "maxbridge/internal/max"
	"maxbridge/internal/storage"
	"maxbridge/internal/telegram"
)

type Server struct {
	cfg       app.Config
	log       *slog.Logger
	metrics   *app.Metrics
	store     *storage.Store
	invites   *invites.Service
	tgClient  *telegram.Client
	maxClient *maxapi.Client
	http      *http.Server
}

func NewServer(
	cfg app.Config,
	log *slog.Logger,
	metrics *app.Metrics,
	store *storage.Store,
	inviteSvc *invites.Service,
	tgClient *telegram.Client,
	maxClient *maxapi.Client,
) *Server {
	mux := http.NewServeMux()
	s := &Server{
		cfg:       cfg,
		log:       log,
		metrics:   metrics,
		store:     store,
		invites:   inviteSvc,
		tgClient:  tgClient,
		maxClient: maxClient,
	}

	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health/live", s.handleLiveness)
	mux.HandleFunc("/health/ready", s.handleReadiness)
	mux.HandleFunc("/health/checks", s.handleChecks)
	mux.HandleFunc("/webhooks/telegram", s.handleTelegramWebhook)
	mux.HandleFunc("/webhooks/max", s.handleMaxWebhook)

	s.http = &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      mux,
		ReadTimeout:  cfg.WebhookReadTimeout,
		WriteTimeout: cfg.WebhookWriteTimeout,
		IdleTimeout:  60 * time.Second,
	}
	return s
}

func (s *Server) Start() error {
	s.log.Info("http server started", "addr", s.cfg.HTTPAddr)
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

func (s *Server) handleLiveness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) handleReadiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.store.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "not_ready", "db": "down"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready", "db": "ok"})
}

func (s *Server) handleChecks(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	checks := map[string]any{"uptime": "ok"}

	if err := s.store.Ping(ctx); err != nil {
		checks["db"] = "fail"
	} else {
		checks["db"] = "ok"
	}
	if err := s.tgClient.Ping(ctx); err != nil {
		checks["telegram"] = "degraded"
	} else {
		checks["telegram"] = "ok"
	}
	if err := s.maxClient.Ping(ctx); err != nil {
		checks["max"] = "degraded"
	} else {
		checks["max"] = "ok"
	}

	stats, err := s.store.GetQueueStats(ctx)
	if err == nil {
		checks["queue"] = map[string]any{
			"pending":     stats.PendingDepth,
			"retry":       stats.RetryDepth,
			"dead_letter": stats.DeadLetterDepth,
		}
	}

	writeJSON(w, http.StatusOK, checks)
}

func (s *Server) handleTelegramWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.metrics.InvalidWebhookTotal.Inc()
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.Header.Get("X-Telegram-Bot-Api-Secret-Token") != s.cfg.TelegramWebhookSecret {
		s.metrics.InvalidWebhookTotal.Inc()
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, s.cfg.MaxWebhookBodyBytes))
	if err != nil {
		s.metrics.InvalidWebhookTotal.Inc()
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var upd domain.TelegramUpdate
	if err := json.Unmarshal(body, &upd); err != nil {
		s.metrics.InvalidWebhookTotal.Inc()
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	s.metrics.TelegramUpdatesTotal.Inc()

	if chat, ok := chatForAutoRegistration(upd); ok {
		title := strings.TrimSpace(chat.Title)
		if title == "" {
			title = "Чат " + strconv.FormatInt(chat.ID, 10)
		}
		if err := s.store.AddTelegramGroup(r.Context(), chat.ID, title); err != nil {
			s.metrics.DBErrorsTotal.Inc()
			s.log.Warn("auto-register telegram group failed", "chat_id", chat.ID, "error", err)
		}
	}
	enqueued, err := s.store.EnqueueTelegramUpdate(r.Context(), upd, s.cfg.WorkerMaxRetry)
	if err != nil {
		s.metrics.DBErrorsTotal.Inc()
		s.log.Error("enqueue telegram update failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if enqueued > 0 {
		s.metrics.TelegramEventsEnqueuedTotal.Add(float64(enqueued))
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func chatForAutoRegistration(upd domain.TelegramUpdate) (domain.TelegramChat, bool) {
	if upd.Message != nil && isTelegramGroupChat(upd.Message.Chat.Type) {
		return upd.Message.Chat, true
	}
	if upd.ChannelPost != nil && isTelegramGroupChat(upd.ChannelPost.Chat.Type) {
		return upd.ChannelPost.Chat, true
	}
	if upd.EditedMessage != nil && isTelegramGroupChat(upd.EditedMessage.Chat.Type) {
		return upd.EditedMessage.Chat, true
	}
	if upd.MyChatMember != nil && isTelegramGroupChat(upd.MyChatMember.Chat.Type) {
		return upd.MyChatMember.Chat, true
	}
	if upd.ChatMember != nil && isTelegramGroupChat(upd.ChatMember.Chat.Type) {
		return upd.ChatMember.Chat, true
	}
	return domain.TelegramChat{}, false
}

func isTelegramGroupChat(chatType string) bool {
	switch chatType {
	case "group", "supergroup":
		return true
	default:
		return false
	}
}

func (s *Server) handleMaxWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.metrics.InvalidWebhookTotal.Inc()
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.Header.Get("X-Max-Bot-Api-Secret") != s.cfg.MaxWebhookSecret {
		s.metrics.InvalidWebhookTotal.Inc()
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, s.cfg.MaxWebhookBodyBytes))
	if err != nil {
		s.metrics.InvalidWebhookTotal.Inc()
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var upd domain.MaxWebhookUpdate
	if err := json.Unmarshal(body, &upd); err != nil {
		s.metrics.InvalidWebhookTotal.Inc()
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if upd.Message == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	userID, text, firstName, lastName, ok := extractMaxLinkInput(upd)
	if !ok {
		s.metrics.InvalidLinkIgnoredTotal.Inc()
		w.WriteHeader(http.StatusNoContent)
		return
	}

	code, ok := invites.ParseLinkCommand(text)
	if !ok {
		s.metrics.InvalidLinkIgnoredTotal.Inc()
		w.WriteHeader(http.StatusNoContent)
		return
	}

	hash := s.invites.HashCode(code)
	inv, err := s.store.ConsumeInvite(r.Context(), hash)
	if err != nil {
		s.metrics.InvalidLinkIgnoredTotal.Inc()
		w.WriteHeader(http.StatusNoContent)
		return
	}

	_, err = s.store.UpsertLinkedUser(r.Context(), userID, firstName, lastName)
	if err != nil {
		s.metrics.DBErrorsTotal.Inc()
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if inv.ScopeType == "group" {
		chatID, convErr := strconv.ParseInt(inv.ScopeID, 10, 64)
		if convErr == nil {
			_, _ = s.store.CreateRoute(r.Context(), chatID, userID, string(domain.RouteFilterAll), true)
		}
	}
	if inv.ScopeType == "route" {
		routeID, convErr := strconv.ParseInt(inv.ScopeID, 10, 64)
		if convErr == nil {
			_ = s.store.CloneRouteToUser(r.Context(), routeID, userID)
		}
	}

	testText := "Bridge link success. Тестовая доставка активирована."
	sendErr := s.maxClient.SendMessage(r.Context(), userID, testText)
	if sendErr != nil {
		_ = s.store.UpdateMaxUserDeliveryStatus(r.Context(), userID, "temporary_error", cutErr(sendErr))
	} else {
		_ = s.store.UpdateMaxUserDeliveryStatus(r.Context(), userID, "success", "")
	}

	s.metrics.SuccessfulLinksTotal.Inc()
	w.WriteHeader(http.StatusNoContent)
}

func extractMaxLinkInput(upd domain.MaxWebhookUpdate) (userID int64, text, firstName, lastName string, ok bool) {
	if upd.Message == nil {
		return 0, "", "", "", false
	}
	userID = upd.Message.Sender.UserID
	if userID <= 0 {
		return 0, "", "", "", false
	}
	firstName = strings.TrimSpace(upd.Message.Sender.FirstName)
	lastName = strings.TrimSpace(upd.Message.Sender.LastName)
	text = strings.TrimSpace(upd.Message.Text)
	if text != "" {
		return userID, text, firstName, lastName, true
	}
	if upd.Message.Body != nil {
		text = strings.TrimSpace(upd.Message.Body.Text)
		if text != "" {
			return userID, text, firstName, lastName, true
		}
	}
	return 0, "", "", "", false
}

func cutErr(err error) string {
	if err == nil {
		return ""
	}
	s := strings.TrimSpace(err.Error())
	if len(s) > 500 {
		return s[:500]
	}
	return s
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
