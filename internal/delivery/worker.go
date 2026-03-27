package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"maxbridge/internal/app"
	"maxbridge/internal/domain"
	maxapi "maxbridge/internal/max"
	"maxbridge/internal/storage"
)

type maxSender interface {
	SendMessage(ctx context.Context, userID int64, text string) error
	SendMessageWithAttachments(ctx context.Context, userID int64, text string, attachments []maxapi.Attachment) error
	UploadAttachment(ctx context.Context, uploadType string, fileName string, content io.Reader) (maxapi.Attachment, error)
}

type telegramMediaFetcher interface {
	DownloadFile(ctx context.Context, fileID string) (io.ReadCloser, string, error)
}

type Worker struct {
	store       *storage.Store
	sender      maxSender
	tgMedia     telegramMediaFetcher
	log         *slog.Logger
	metrics     *app.Metrics
	concurrency int
	lease       time.Duration
	maxRetry    int
	limiter     *rate.Limiter
	breaker     *CircuitBreaker
}

func NewWorker(
	store *storage.Store,
	sender maxSender,
	tgMedia telegramMediaFetcher,
	log *slog.Logger,
	metrics *app.Metrics,
	concurrency int,
	lease time.Duration,
	maxRetry int,
	rateLimitRPS int,
) *Worker {
	if concurrency < 1 {
		concurrency = 1
	}
	if rateLimitRPS < 1 {
		rateLimitRPS = 1
	}
	return &Worker{
		store:       store,
		sender:      sender,
		tgMedia:     tgMedia,
		log:         log,
		metrics:     metrics,
		concurrency: concurrency,
		lease:       lease,
		maxRetry:    maxRetry,
		limiter:     rate.NewLimiter(rate.Limit(rateLimitRPS), rateLimitRPS),
		breaker:     NewCircuitBreaker(20, 20*time.Second),
	}
}

func (w *Worker) Run(ctx context.Context) error {
	jobsCh := make(chan domain.DeliveryJob, w.concurrency*2)
	var wg sync.WaitGroup

	for i := 0; i < w.concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-jobsCh:
					if !ok {
						return
					}
					w.processOne(ctx, job)
				}
			}
		}(i)
	}

	claimTicker := time.NewTicker(700 * time.Millisecond)
	defer claimTicker.Stop()

	staleTicker := time.NewTicker(15 * time.Second)
	defer staleTicker.Stop()

	statsTicker := time.NewTicker(5 * time.Second)
	defer statsTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			close(jobsCh)
			wg.Wait()
			return nil
		case <-claimTicker.C:
			jobs, err := w.store.ClaimJobs(ctx, w.concurrency, w.lease)
			if err != nil {
				w.metrics.DBErrorsTotal.Inc()
				w.log.Error("claim jobs failed", "error", err)
				continue
			}
			for _, j := range jobs {
				select {
				case jobsCh <- j:
				case <-ctx.Done():
					close(jobsCh)
					wg.Wait()
					return nil
				}
			}
		case <-staleTicker.C:
			requeued, err := w.store.RequeueStaleProcessing(ctx)
			if err != nil {
				w.log.Error("requeue stale failed", "error", err)
				w.metrics.DBErrorsTotal.Inc()
			} else if requeued > 0 {
				w.log.Warn("stale jobs requeued", "count", requeued)
			}
		case <-statsTicker.C:
			st, err := w.store.GetQueueStats(ctx)
			if err == nil {
				w.metrics.QueueDepth.Set(float64(st.PendingDepth + st.RetryDepth))
				w.metrics.OldestPendingJobAge.Set(float64(st.OldestPendingAgeS))
			}
		}
	}
}

func (w *Worker) processOne(ctx context.Context, job domain.DeliveryJob) {
	if !w.breaker.Allow() {
		w.retry(ctx, job, errors.New("circuit breaker open"), true)
		return
	}
	if err := w.limiter.Wait(ctx); err != nil {
		w.retry(ctx, job, err, true)
		return
	}

	out := renderMessage(job)
	attachments, mediaErr := w.prepareAttachments(ctx, out.Media)
	if mediaErr != nil {
		w.retry(ctx, job, mediaErr, maxapi.IsTemporarySendError(mediaErr))
		return
	}
	start := time.Now()
	err := w.sendWithAttachmentWarmup(ctx, job.MaxUserID, out.Text, attachments)
	latency := time.Since(start)
	w.metrics.MaxAPILatency.Observe(latency.Seconds())

	if err == nil {
		w.breaker.MarkSuccess()
		if recErr := w.store.RecordAttempt(ctx, job.ID, domain.DeliverySuccess, "", "", latency.Milliseconds()); recErr != nil {
			w.log.Error("record attempt success failed", "job_id", job.ID, "error", recErr)
		}
		if err := w.store.MarkJobCompleted(ctx, job.ID); err != nil {
			w.log.Error("mark completed failed", "job_id", job.ID, "error", err)
			w.metrics.DBErrorsTotal.Inc()
		}
		_ = w.store.UpdateMaxUserDeliveryStatus(ctx, job.MaxUserID, "success", "")
		_ = w.store.UpdateRouteDeliveryStatus(ctx, job.RouteID, "success", "")
		w.metrics.MaxSendSuccessTotal.Inc()
		return
	}

	temporary := maxapi.IsTemporarySendError(err)
	w.breaker.MarkFailure()
	if temporary {
		w.retry(ctx, job, err, true)
		return
	}
	w.retry(ctx, job, err, false)
}

func (w *Worker) sendWithAttachmentWarmup(ctx context.Context, maxUserID int64, text string, attachments []maxapi.Attachment) error {
	err := w.sender.SendMessageWithAttachments(ctx, maxUserID, text, attachments)
	if err == nil || len(attachments) == 0 || !maxapi.IsAttachmentNotReadyError(err) {
		return err
	}

	delays := []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second}
	lastErr := err
	for _, d := range delays {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d):
		}
		lastErr = w.sender.SendMessageWithAttachments(ctx, maxUserID, text, attachments)
		if lastErr == nil {
			return nil
		}
		if !maxapi.IsAttachmentNotReadyError(lastErr) {
			return lastErr
		}
	}
	return lastErr
}

func (w *Worker) retry(ctx context.Context, job domain.DeliveryJob, err error, temporary bool) {
	nextAttempt := job.Attempts + 1
	nextTime := time.Now().UTC()
	result := domain.DeliveryPermanent
	errorClass := "permanent"
	if temporary {
		result = domain.DeliveryTemporary
		errorClass = "temporary"
		nextTime = nextTime.Add(NextBackoff(nextAttempt))
	}

	_ = w.store.RecordAttempt(ctx, job.ID, result, errorClass, cutErr(err), 0)
	status, dbErr := w.store.MarkJobRetryOrDead(ctx, job.ID, nextAttempt, maxInt(job.MaxAttempts, w.maxRetry), nextTime, cutErr(err))
	if dbErr != nil {
		w.log.Error("mark retry/dead failed", "job_id", job.ID, "error", dbErr)
		w.metrics.DBErrorsTotal.Inc()
		return
	}
	_ = w.store.UpdateMaxUserDeliveryStatus(ctx, job.MaxUserID, string(result), cutErr(err))
	_ = w.store.UpdateRouteDeliveryStatus(ctx, job.RouteID, string(result), cutErr(err))

	if status == domain.JobDeadLetter {
		w.metrics.DeadLetterTotal.Inc()
		w.metrics.MaxSendFailureTotal.Inc()
		return
	}
	w.metrics.RetryTotal.Inc()
}

type outboundMedia struct {
	FileID         string
	FileName       string
	AttachmentType string
	UploadType     string
}

type outboundMessage struct {
	Text  string
	Media []outboundMedia
}

func renderMessage(job domain.DeliveryJob) outboundMessage {
	var msg domain.TelegramMessage
	if err := json.Unmarshal(job.PayloadJSON, &msg); err != nil {
		return outboundMessage{
			Text: fmt.Sprintf("[bridge] tg:%d msg:%d", job.TelegramChatID, job.TelegramMessageID),
		}
	}
	chatName := telegramChatName(msg.Chat)
	senderName := telegramSenderName(msg.From)
	header := fmt.Sprintf("%s - %s", chatName, senderName)
	bodyParts := make([]string, 0, 2)

	if text := strings.TrimSpace(msg.Text); text != "" {
		bodyParts = append(bodyParts, text)
	}
	if caption := strings.TrimSpace(msg.Caption); caption != "" {
		bodyParts = append(bodyParts, caption)
	}

	media := extractOutboundMedia(msg)
	if len(bodyParts) == 0 && len(media) == 0 {
		bodyParts = append(bodyParts, fmt.Sprintf("[bridge] сообщение без текста и вложений\nchat=%d msg=%d", msg.Chat.ID, msg.MessageID))
	}
	if len(bodyParts) == 0 {
		return outboundMessage{Text: header, Media: media}
	}
	return outboundMessage{
		Text:  header + "\n" + strings.Join(bodyParts, "\n"),
		Media: media,
	}
}

func telegramChatName(chat domain.TelegramChat) string {
	if title := strings.TrimSpace(chat.Title); title != "" {
		return title
	}
	if username := strings.TrimSpace(chat.Username); username != "" {
		return "@" + strings.TrimPrefix(username, "@")
	}
	return fmt.Sprintf("chat_%d", chat.ID)
}

func telegramSenderName(from *domain.TelegramUser) string {
	if from == nil {
		return "unknown_sender"
	}
	fullName := strings.TrimSpace(strings.TrimSpace(from.FirstName) + " " + strings.TrimSpace(from.LastName))
	if fullName != "" {
		return fullName
	}
	if username := strings.TrimSpace(from.Username); username != "" {
		return "@" + strings.TrimPrefix(username, "@")
	}
	if from.ID > 0 {
		return fmt.Sprintf("user_%d", from.ID)
	}
	return "unknown_sender"
}

func extractOutboundMedia(msg domain.TelegramMessage) []outboundMedia {
	out := make([]outboundMedia, 0, 1)

	if len(msg.Photo) > 0 {
		last := msg.Photo[len(msg.Photo)-1]
		if strings.TrimSpace(last.FileID) != "" {
			out = append(out, outboundMedia{
				FileID:         last.FileID,
				FileName:       "photo.jpg",
				AttachmentType: "image",
				UploadType:     "image",
			})
		}
	}
	if msg.Document != nil && strings.TrimSpace(msg.Document.FileID) != "" {
		out = append(out, outboundMedia{
			FileID:         msg.Document.FileID,
			FileName:       msg.Document.FileName,
			AttachmentType: "file",
			UploadType:     "file",
		})
	}
	if msg.Video != nil && strings.TrimSpace(msg.Video.FileID) != "" {
		out = append(out, outboundMedia{
			FileID:         msg.Video.FileID,
			FileName:       "video.mp4",
			AttachmentType: "video",
			UploadType:     "video",
		})
	}
	if msg.Audio != nil && strings.TrimSpace(msg.Audio.FileID) != "" {
		name := strings.TrimSpace(msg.Audio.FileName)
		if name == "" {
			name = "audio.mp3"
		}
		out = append(out, outboundMedia{
			FileID:         msg.Audio.FileID,
			FileName:       name,
			AttachmentType: "audio",
			UploadType:     "audio",
		})
	}
	if msg.Voice != nil && strings.TrimSpace(msg.Voice.FileID) != "" {
		out = append(out, outboundMedia{
			FileID:         msg.Voice.FileID,
			FileName:       "voice.ogg",
			AttachmentType: "file",
			UploadType:     "file",
		})
	}
	if msg.Animation != nil && strings.TrimSpace(msg.Animation.FileID) != "" {
		name := strings.TrimSpace(msg.Animation.FileName)
		if name == "" {
			name = "animation.gif"
		}
		out = append(out, outboundMedia{
			FileID:         msg.Animation.FileID,
			FileName:       name,
			AttachmentType: "file",
			UploadType:     "file",
		})
	}
	return out
}

func (w *Worker) prepareAttachments(ctx context.Context, media []outboundMedia) ([]maxapi.Attachment, error) {
	if len(media) == 0 {
		return nil, nil
	}
	if w.tgMedia == nil {
		return nil, errors.New("telegram media client is not configured")
	}

	out := make([]maxapi.Attachment, 0, len(media))
	for _, m := range media {
		body, fallbackName, err := w.tgMedia.DownloadFile(ctx, m.FileID)
		if err != nil {
			return nil, err
		}

		filename := strings.TrimSpace(m.FileName)
		if filename == "" {
			filename = strings.TrimSpace(fallbackName)
		}
		if filename == "" {
			filename = "attachment.bin"
		}
		filename = filepath.Base(filename)

		att, upErr := w.sender.UploadAttachment(ctx, m.UploadType, filename, body)
		_ = body.Close()
		if upErr != nil {
			return nil, upErr
		}
		att.Type = m.AttachmentType
		out = append(out, att)
	}
	return out, nil
}

func cutErr(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if len(s) > 500 {
		return s[:500]
	}
	return s
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
