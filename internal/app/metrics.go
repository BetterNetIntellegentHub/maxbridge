package app

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	TelegramUpdatesTotal        prometheus.Counter
	TelegramEventsEnqueuedTotal prometheus.Counter
	MaxSendSuccessTotal         prometheus.Counter
	MaxSendFailureTotal         prometheus.Counter
	RetryTotal                  prometheus.Counter
	DeadLetterTotal             prometheus.Counter
	QueueDepth                  prometheus.Gauge
	OldestPendingJobAge         prometheus.Gauge
	MaxAPILatency               prometheus.Histogram
	DBErrorsTotal               prometheus.Counter
	InvalidWebhookTotal         prometheus.Counter
	InvalidLinkIgnoredTotal     prometheus.Counter
	SuccessfulLinksTotal        prometheus.Counter
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		TelegramUpdatesTotal: prometheus.NewCounter(prometheus.CounterOpts{Name: "telegram_updates_total", Help: "Telegram updates received"}),
		TelegramEventsEnqueuedTotal: prometheus.NewCounter(prometheus.CounterOpts{Name: "telegram_events_enqueued_total", Help: "Telegram events enqueued"}),
		MaxSendSuccessTotal:         prometheus.NewCounter(prometheus.CounterOpts{Name: "max_send_success_total", Help: "MAX successful sends"}),
		MaxSendFailureTotal:         prometheus.NewCounter(prometheus.CounterOpts{Name: "max_send_failure_total", Help: "MAX failed sends"}),
		RetryTotal:                  prometheus.NewCounter(prometheus.CounterOpts{Name: "retry_total", Help: "Retry count"}),
		DeadLetterTotal:             prometheus.NewCounter(prometheus.CounterOpts{Name: "dead_letter_total", Help: "Dead-lettered jobs"}),
		QueueDepth:                  prometheus.NewGauge(prometheus.GaugeOpts{Name: "queue_depth", Help: "Queue depth"}),
		OldestPendingJobAge:         prometheus.NewGauge(prometheus.GaugeOpts{Name: "oldest_pending_job_age", Help: "Oldest pending job age seconds"}),
		MaxAPILatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "max_api_latency",
			Help:    "MAX API latency in seconds",
			Buckets: prometheus.DefBuckets,
		}),
		DBErrorsTotal:           prometheus.NewCounter(prometheus.CounterOpts{Name: "db_errors_total", Help: "DB errors"}),
		InvalidWebhookTotal:     prometheus.NewCounter(prometheus.CounterOpts{Name: "invalid_webhook_total", Help: "Invalid webhook requests"}),
		InvalidLinkIgnoredTotal: prometheus.NewCounter(prometheus.CounterOpts{Name: "invalid_link_ignored_total", Help: "Ignored invalid links"}),
		SuccessfulLinksTotal:    prometheus.NewCounter(prometheus.CounterOpts{Name: "successful_links_total", Help: "Successful link operations"}),
	}

	reg.MustRegister(
		m.TelegramUpdatesTotal,
		m.TelegramEventsEnqueuedTotal,
		m.MaxSendSuccessTotal,
		m.MaxSendFailureTotal,
		m.RetryTotal,
		m.DeadLetterTotal,
		m.QueueDepth,
		m.OldestPendingJobAge,
		m.MaxAPILatency,
		m.DBErrorsTotal,
		m.InvalidWebhookTotal,
		m.InvalidLinkIgnoredTotal,
		m.SuccessfulLinksTotal,
	)

	return m
}
