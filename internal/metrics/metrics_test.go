package metrics

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/KamorionLabs/aws-smtp-relay/internal/relay"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

var origin = &net.TCPAddr{IP: net.ParseIP("127.0.0.1")}

func TestHandlerCountsResults(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := New(reg)
	for _, err := range []error{
		nil,
		nil,
		relay.ErrDeniedSender,
		fmt.Errorf("send: %w", relay.ErrDeniedRecipients),
		errors.New("MessageRejected"),
	} {
		handler := m.Handler(func(net.Addr, string, []string, []byte) error { return err })
		if got := handler(origin, "alice@example.org", nil, nil); got != err {
			t.Errorf("handler returned %v, want %v", got, err)
		}
	}
	for result, want := range map[string]float64{resultSent: 2, resultDenied: 2, resultError: 1} {
		if got := testutil.ToFloat64(m.messages.WithLabelValues(result)); got != want {
			t.Errorf("%s = %v, want %v", result, got, want)
		}
	}
	if got := sampleCount(t, reg, "aws_smtp_relay_send_duration_seconds"); got != 5 {
		t.Errorf("send durations = %d, want 5", got)
	}
}

func TestAuthHandlerCountsFailures(t *testing.T) {
	m := New(prometheus.NewRegistry())
	for _, tc := range []struct {
		ok  bool
		err error
	}{{true, nil}, {false, nil}, {false, errors.New("Invalid username")}} {
		auth := m.AuthHandler(func(net.Addr, string, []byte, []byte, []byte) (bool, error) {
			return tc.ok, tc.err
		})
		if ok, err := auth(origin, "PLAIN", nil, nil, nil); ok != tc.ok || err != tc.err {
			t.Errorf("auth returned %v, %v; want %v, %v", ok, err, tc.ok, tc.err)
		}
	}
	if got := testutil.ToFloat64(m.authFailures); got != 2 {
		t.Errorf("auth failures = %v, want 2", got)
	}
}

func TestServe(t *testing.T) {
	reg := prometheus.NewRegistry()
	New(reg)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go Serve(ln, reg)
	base := "http://" + ln.Addr().String()
	if body := get(t, base+"/healthz"); body != "ok\n" {
		t.Errorf("/healthz = %q", body)
	}
	if body := get(t, base+"/metrics"); !strings.Contains(body, "aws_smtp_relay_auth_failures_total 0") {
		t.Errorf("/metrics has no relay metrics:\n%s", body)
	}
}

func sampleCount(t *testing.T, g prometheus.Gatherer, name string) uint64 {
	t.Helper()
	families, err := g.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() == name {
			return f.GetMetric()[0].GetHistogram().GetSampleCount()
		}
	}
	t.Fatalf("no metric %s", name)
	return 0
}

func get(t *testing.T, url string) string {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: status %d", url, resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	return string(body)
}
