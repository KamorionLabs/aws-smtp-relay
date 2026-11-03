package main

import (
	"net"
	"strings"
	"testing"

	"github.com/mhale/smtpd"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestStartMetrics(t *testing.T) {
	srv := &smtpd.Server{
		Handler: func(net.Addr, string, []string, []byte) error { return nil },
		AuthHandler: func(net.Addr, string, []byte, []byte, []byte) (bool, error) {
			return false, nil
		},
	}
	reg := prometheus.NewRegistry()
	if err := startMetrics(srv, "127.0.0.1:0", reg, reg); err != nil {
		t.Fatal(err)
	}
	origin := &net.TCPAddr{IP: net.ParseIP("127.0.0.1")}
	srv.Handler(origin, "alice@example.org", []string{"bob@example.org"}, nil)
	srv.AuthHandler(origin, "PLAIN", []byte("alice"), []byte("wrong"), nil)
	want := `
# HELP aws_smtp_relay_auth_failures_total Failed SMTP authentications.
# TYPE aws_smtp_relay_auth_failures_total counter
aws_smtp_relay_auth_failures_total 1
# HELP aws_smtp_relay_messages_total Messages by result: sent, denied (sender or recipient filter) or error (API error).
# TYPE aws_smtp_relay_messages_total counter
aws_smtp_relay_messages_total{result="denied"} 0
aws_smtp_relay_messages_total{result="error"} 0
aws_smtp_relay_messages_total{result="sent"} 1
`
	err := testutil.GatherAndCompare(reg, strings.NewReader(want),
		"aws_smtp_relay_auth_failures_total", "aws_smtp_relay_messages_total")
	if err != nil {
		t.Error(err)
	}
}

func TestStartMetricsWithInvalidAddress(t *testing.T) {
	reg := prometheus.NewRegistry()
	if err := startMetrics(&smtpd.Server{}, "invalid address", reg, reg); err == nil {
		t.Error("want an error for an invalid address")
	}
}
