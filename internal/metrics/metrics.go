/*
Package metrics counts the messages and failed logins of the relay and serves
the counts for Prometheus.
*/
package metrics

import (
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/KamorionLabs/aws-smtp-relay/internal/relay"
	"github.com/mhale/smtpd"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Results of a send, the values of the "result" label.
const (
	resultSent   = "sent"
	resultDenied = "denied"
	resultError  = "error"
)

// Metrics holds the relay metrics.
type Metrics struct {
	messages     *prometheus.CounterVec
	authFailures prometheus.Counter
	sendDuration prometheus.Histogram
}

// New makes the relay metrics and registers them with reg.
func New(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		messages: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "aws_smtp_relay_messages_total",
			Help: "Messages by result: sent, denied (sender or recipient filter) or error (API error).",
		}, []string{"result"}),
		authFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "aws_smtp_relay_auth_failures_total",
			Help: "Failed SMTP authentications.",
		}),
		sendDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "aws_smtp_relay_send_duration_seconds",
			Help:    "Time to handle a message, with the API request when there is one.",
			Buckets: prometheus.DefBuckets,
		}),
	}
	// Start each result at 0, so that rate() sees the first message.
	for _, r := range []string{resultSent, resultDenied, resultError} {
		m.messages.WithLabelValues(r)
	}
	reg.MustRegister(m.messages, m.authFailures, m.sendDuration)
	return m
}

// Handler counts the result of each message that next handles.
func (m *Metrics) Handler(next smtpd.Handler) smtpd.Handler {
	return func(origin net.Addr, from string, to []string, data []byte) error {
		start := time.Now()
		err := next(origin, from, to, data)
		m.sendDuration.Observe(time.Since(start).Seconds())
		m.messages.WithLabelValues(sendResult(err)).Inc()
		return err
	}
}

func sendResult(err error) string {
	switch {
	case err == nil:
		return resultSent
	case errors.Is(err, relay.ErrDeniedSender), errors.Is(err, relay.ErrDeniedRecipients):
		return resultDenied
	default:
		return resultError
	}
}

// AuthHandler counts the failed authentications of next.
func (m *Metrics) AuthHandler(next smtpd.AuthHandler) smtpd.AuthHandler {
	return func(origin net.Addr, mechanism string, username, password, shared []byte) (bool, error) {
		ok, err := next(origin, mechanism, username, password, shared)
		if !ok {
			m.authFailures.Inc()
		}
		return ok, err
	}
}

// Serve serves the metrics of g at /metrics, and at /healthz a check that the
// process runs.
func Serve(ln net.Listener, g prometheus.Gatherer) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(g, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok\n"))
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	return srv.Serve(ln)
}
