package main

import (
	"errors"
	"net"
	"net/smtp"
	"net/textproto"
	"testing"

	"github.com/KamorionLabs/aws-smtp-relay/internal/relay"
)

const testMessage = "Subject: test\r\n\r\nbody\r\n"

type fakeRelay struct {
	err error
	to  []string
}

func (f *fakeRelay) Send(origin net.Addr, from string, to []string, data []byte) error {
	f.to = to
	return f.err
}

// startServer runs the configured server with fake as relay client.
func startServer(t *testing.T, fake *fakeRelay) string {
	t.Helper()
	if err := configure(); err != nil {
		t.Fatal(err)
	}
	relayClient = fake
	srv, err := server()
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(ln)
	t.Cleanup(func() {
		srv.Close()
		ln.Close()
	})
	return ln.Addr().String()
}

func replyCode(t *testing.T, err error) int {
	t.Helper()
	var protoErr *textproto.Error
	if !errors.As(err, &protoErr) {
		t.Fatalf("not an SMTP reply: %v", err)
	}
	return protoErr.Code
}

func TestServerReplyCodes(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code int
	}{
		{"sent", nil, 250},
		{"denied sender", relay.ErrDeniedSender, 550},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetHelper()
			addr := startServer(t, &fakeRelay{err: tc.err})
			err := smtp.SendMail(addr, nil, "alice@example.org",
				[]string{"carol@example.org"}, []byte(testMessage))
			code := 250
			if err != nil {
				code = replyCode(t, err)
			}
			if code != tc.code {
				t.Errorf("want %d, got %d", tc.code, code)
			}
		})
	}
}

func TestServerRefusesDeniedRecipientAtRcpt(t *testing.T) {
	resetHelper()
	*denyTo = "^bob@"
	fake := &fakeRelay{}
	c, err := smtp.Dial(startServer(t, fake))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.Mail("alice@example.org"); err != nil {
		t.Fatal(err)
	}
	if err := c.Rcpt("carol@example.org"); err != nil {
		t.Fatal(err)
	}
	if code := replyCode(t, c.Rcpt("bob@example.org")); code != 550 {
		t.Errorf("want 550 at RCPT, got %d", code)
	}
	w, err := c.Data()
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte(testMessage))
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if len(fake.to) != 1 || fake.to[0] != "carol@example.org" {
		t.Errorf("the relay was called for %v, want [carol@example.org]", fake.to)
	}
}
