package relay

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"regexp"
	"testing"
	"time"
)

func logHelper(addr net.Addr, from string, to []string, err error) (
	[]byte,
	[]byte,
) {
	outReader, outWriter, _ := os.Pipe()
	errReader, errWriter, _ := os.Pipe()
	originalOut := os.Stdout
	originalErr := os.Stderr
	defer func() {
		os.Stdout = originalOut
		os.Stderr = originalErr
	}()
	os.Stdout = outWriter
	os.Stderr = errWriter
	func() {
		Log(addr, from, to, err)
		outWriter.Close()
		errWriter.Close()
	}()
	stdout, _ := io.ReadAll(outReader)
	stderr, _ := io.ReadAll(errReader)
	return stdout, stderr
}

func TestLog(t *testing.T) {
	origin := net.TCPAddr{IP: []byte{127, 0, 0, 1}}
	emails := []string{
		"alice@example.org",
		"bob@example.org",
		"charlie@example.org",
	}
	from := emails[0]
	to := []string{emails[1], emails[2]}
	timeBefore := time.Now()
	out, err := logHelper(&origin, from, to, nil)
	timeAfter := time.Now()
	var entry logEntry
	json.Unmarshal(out, &entry)
	if entry.Time.Before(timeBefore) {
		t.Errorf("Unexpected 'Time' log: %s", entry.Time)
	}
	if entry.Time.After(timeAfter) {
		t.Errorf("Unexpected 'Time' log: %s", entry.Time)
	}
	if entry.IP == "" {
		t.Errorf("Unexpected 'IP' log: %v. Expected: %s", nil, "127.0.0.1")
	} else if entry.IP != "127.0.0.1" {
		t.Errorf("Unexpected 'IP' log: %s. Expected: %s", entry.IP, "127.0.0.1")
	}
	if entry.From == "" {
		t.Errorf("Unexpected 'From' log: %v. Expected: %s", nil, from)
	} else if entry.From != from {
		t.Errorf("Unexpected 'From' log: %s. Expected: %s", entry.From, from)
	}
	toVals := entry.To
	expectedToVals := to
	if len(toVals) != len(expectedToVals) ||
		toVals[0] != expectedToVals[0] || toVals[1] != expectedToVals[1] {
		t.Errorf("Unexpected 'To' log: %s. Expected: %s", toVals, expectedToVals)
	}
	if entry.Error != nil {
		t.Errorf("Unexpected 'Error' log: %s. Expected: %v", *entry.Error, nil)
	}
	if len(err) != 0 {
		t.Errorf("Unexpected stderr: %s", err)
	}
}

func TestLogWithOriginIPv6(t *testing.T) {
	origin := net.TCPAddr{IP: []byte{
		0x20, 0x01, 0x48, 0x60, 0, 0, 0x20, 0x01, 0, 0, 0, 0, 0, 0, 0x00, 0x68,
	}}
	emails := []string{
		"alice@example.org",
		"bob@example.org",
		"charlie@example.org",
	}
	from := emails[0]
	to := []string{emails[1], emails[2]}
	out, err := logHelper(&origin, from, to, nil)
	var entry logEntry
	json.Unmarshal(out, &entry)
	if entry.IP != "2001:4860:0:2001::68" {
		t.Errorf(
			"Unexpected 'IP' log: %s. Expected: %s",
			entry.IP,
			"2001:4860:0:2001::68",
		)
	}
	if entry.Error != nil {
		t.Errorf("Unexpected 'Error' log: %s. Expected: %v", *entry.Error, nil)
	}
	if len(err) != 0 {
		t.Errorf("Unexpected stderr: %s", err)
	}
}

func TestLogWithError(t *testing.T) {
	origin := net.TCPAddr{IP: []byte{127, 0, 0, 1}}
	emails := []string{
		"alice@example.org",
		"bob@example.org",
		"charlie@example.org",
	}
	from := emails[0]
	to := []string{emails[1], emails[2]}
	out, err := logHelper(&origin, from, to, errors.New("ERROR"))
	var entry logEntry
	json.Unmarshal(out, &entry)
	if entry.Error == nil {
		t.Errorf("Unexpected 'Error' log: %v. Expected: %s", nil, "ERROR")
	} else if *entry.Error != "ERROR" {
		t.Errorf("Unexpected 'Error' log: %s. Expected: %s", *entry.Error, "ERROR")
	}
	if len(err) != 0 {
		t.Errorf("Unexpected stderr: %s", err)
	}
}

func TestFilterAddresses(t *testing.T) {
	from := "alice@example.org"
	to := []string{
		"bob@example.org",
		"charlie@example.org",
	}
	allowedRecipients, deniedRecipients, err := FilterAddresses(
		from,
		to,
		nil,
		nil,
	)
	if err != nil {
		t.Errorf("Unexpected error: %s. Expected: %v", err, nil)
	}
	allowedRecipientsValues := allowedRecipients
	if len(allowedRecipients) != 2 || allowedRecipientsValues[0] != to[0] ||
		allowedRecipientsValues[1] != to[1] {
		t.Errorf(
			"Unexpected allowed recipients: %s. Expected: %s",
			allowedRecipientsValues,
			to,
		)
	}
	if len(deniedRecipients) != 0 {
		t.Errorf(
			"Unexpected denied recipients: %s. Expected: %s",
			deniedRecipients,
			[]string{},
		)
	}
}

func TestFilterAddressesWithDeniedSender(t *testing.T) {
	from := "alice@example.org"
	to := []string{
		"bob@example.org",
		"charlie@example.org",
	}
	allowFromRegExp, _ := regexp.Compile(`^admin@example\.org$`)
	denyToRegExp, _ := regexp.Compile("^david@example.org$")
	expectedError := ErrDeniedSender
	allowedRecipients, deniedRecipients, err := FilterAddresses(
		from,
		to,
		allowFromRegExp,
		denyToRegExp,
	)
	if err == nil {
		t.Errorf("Unexpected error: %v. Expected: %s", nil, expectedError)
	} else if err.Error() != expectedError.Error() {
		t.Errorf(
			"Unexpected error: `%s`. Expected: `%s`",
			err.Error(),
			expectedError.Error(),
		)
	}
	if len(allowedRecipients) != 0 {
		t.Errorf(
			"Unexpected allowed recipients: %s. Expected: %s",
			allowedRecipients,
			[]string{},
		)
	}
	deniedRecipientsValues := deniedRecipients
	if len(deniedRecipients) != 2 || deniedRecipientsValues[0] != to[0] ||
		deniedRecipientsValues[1] != to[1] {
		t.Errorf(
			"Unexpected denied recipients: %s. Expected: %s",
			deniedRecipientsValues,
			to,
		)
	}
}

func TestFilterAddressesWithDeniedRecipients(t *testing.T) {
	from := "alice@example.org"
	to := []string{
		"bob@example.org",
		"charlie@example.org",
	}
	allowFromRegExp, _ := regexp.Compile(`^[^@]+@example\.org$`)
	denyToRegExp, _ := regexp.Compile(`^bob@example\.org$`)
	expectedError := ErrDeniedRecipients
	allowedRecipients, deniedRecipients, err := FilterAddresses(
		from,
		to,
		allowFromRegExp,
		denyToRegExp,
	)
	if err == nil {
		t.Errorf("Unexpected error: %v. Expected: %s", nil, expectedError)
	} else if err.Error() != expectedError.Error() {
		t.Errorf(
			"Unexpected error: `%s`. Expected: `%s`",
			err.Error(),
			expectedError.Error(),
		)
	}
	allowedRecipientsValues := allowedRecipients
	if len(allowedRecipients) != 1 || allowedRecipientsValues[0] != to[1] {
		t.Errorf(
			"Unexpected allowed recipients: %s. Expected: %s",
			allowedRecipientsValues,
			[]string{"charlie@example.org"},
		)
	}
	deniedRecipientsValues := deniedRecipients
	if len(deniedRecipients) != 1 || deniedRecipientsValues[0] != to[0] {
		t.Errorf(
			"Unexpected denied recipients: %s. Expected: %s",
			deniedRecipientsValues,
			[]string{"bob@example.org"},
		)
	}
}

func TestFilterCheckRecipient(t *testing.T) {
	f := &Filter{
		DenyTo:         regexp.MustCompile(`^admin@`),
		AllowTo:        regexp.MustCompile(`^[a-z]+@`),
		AllowToDomains: ParseDomains("example.org, Example.NET"),
	}
	cases := []struct {
		to   string
		want error
	}{
		{"bob@example.org", nil},
		{"bob@EXAMPLE.net", nil},
		{"admin@example.org", ErrDeniedRecipients},
		{"bob2@example.org", ErrRecipientsNotAllowed},
		{"bob@example.com", ErrRecipientsNotAllowed},
		{"bob@sub.example.org", ErrRecipientsNotAllowed},
		{"bob@example.org@evil.com", ErrRecipientsNotAllowed},
		{"bob", ErrRecipientsNotAllowed},
	}
	for _, tc := range cases {
		if got := f.CheckRecipient(tc.to); got != tc.want {
			t.Errorf("CheckRecipient(%q) = %v, want %v", tc.to, got, tc.want)
		}
	}
	var nilFilter *Filter
	if err := nilFilter.CheckRecipient("anyone@example.com"); err != nil {
		t.Errorf("nil filter: %v", err)
	}
}

func TestErrRecipientsNotAllowedIsDeniedRecipients(t *testing.T) {
	if !errors.Is(ErrRecipientsNotAllowed, ErrDeniedRecipients) {
		t.Error("ErrRecipientsNotAllowed does not match ErrDeniedRecipients")
	}
	if errors.Is(ErrDeniedRecipients, ErrRecipientsNotAllowed) {
		t.Error("ErrDeniedRecipients matches ErrRecipientsNotAllowed")
	}
}

func TestFilterAddressesWithAllowLists(t *testing.T) {
	f := &Filter{AllowToDomains: []string{"example.org"}}
	allowed, denied, err := f.Addresses(
		"alice@example.org",
		[]string{"bob@example.org", "carol@example.com"},
	)
	if err != ErrRecipientsNotAllowed {
		t.Errorf("Unexpected error: %v. Expected: %v", err, ErrRecipientsNotAllowed)
	}
	if len(allowed) != 1 || allowed[0] != "bob@example.org" {
		t.Errorf("Unexpected allowed recipients: %v", allowed)
	}
	if len(denied) != 1 || denied[0] != "carol@example.com" {
		t.Errorf("Unexpected denied recipients: %v", denied)
	}
}

func TestParseDomains(t *testing.T) {
	got := ParseDomains(" Example.org,,@example.net , ")
	if len(got) != 2 || got[0] != "example.org" || got[1] != "example.net" {
		t.Errorf("Unexpected domains: %v", got)
	}
	if ParseDomains("") != nil {
		t.Error("Expected nil for an empty list")
	}
}
