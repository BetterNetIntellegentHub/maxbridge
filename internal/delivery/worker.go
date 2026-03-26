package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
}

type Worker struct {
	store       *storage.Store
	sender      maxSender
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

	text := renderMessage(job)
	start := time.Now()
	err := w.sender.SendMessage(ctx, job.MaxUserID, text)
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

func renderMessage(job domain.DeliveryJob) string {
	var msg domain.TelegramMessage
	if err := json.Unmarshal(job.PayloadJSON, &msg); err != nil {
		return fmt.Sprintf("[bridge] tg:%d msg:%d", job.TelegramChatID, job.TelegramMessageID)
	}
	if msg.Text == "" {
		return fmt.Sprintf("[bridge] сообщение без текста\nchat=%d msg=%d", msg.Chat.ID, msg.MessageID)
	}
	return fmt.Sprintf("[TG %d] %s", msg.Chat.ID, msg.Text)
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

