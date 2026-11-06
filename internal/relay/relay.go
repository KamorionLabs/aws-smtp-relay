/*
Package relay provides an interface to relay emails via Amazon SES/Pinpoint API.
*/
package relay

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

var (
	ErrDeniedSender = errors.New(
		"denied sender: sender does not match the allowed emails regexp",
	)

	ErrDeniedRecipients = errors.New(
		"denied recipients: recipients match the denied emails regexp",
	)

	// ErrRecipientsNotAllowed matches ErrDeniedRecipients with errors.Is, so
	// that callers can handle every denied recipient the same way.
	ErrRecipientsNotAllowed error = &notAllowedError{
		"denied recipients: recipients do not match the allowed emails regexp or domains",
	}
)

type notAllowedError struct{ msg string }

func (e *notAllowedError) Error() string { return e.msg }

func (e *notAllowedError) Is(target error) bool { return target == ErrDeniedRecipients }

// Filter holds the sender and recipient filters. A nil field means no
// restriction.
type Filter struct {
	// AllowFrom is an allow list of sender addresses.
	AllowFrom *regexp.Regexp
	// DenyTo is a deny list of recipient addresses.
	DenyTo *regexp.Regexp
	// AllowTo is an allow list of recipient addresses.
	AllowTo *regexp.Regexp
	// AllowToDomains is an allow list of recipient domains, in lower case.
	AllowToDomains []string
}

// ParseDomains splits a comma-separated list of domains, trims spaces and a
// leading "@", lowercases them and drops empty entries.
func ParseDomains(list string) []string {
	var domains []string
	for _, d := range strings.Split(list, ",") {
		d = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(d), "@"))
		if d != "" {
			domains = append(domains, d)
		}
	}
	return domains
}

// CheckRecipient returns nil if the recipient is allowed, ErrDeniedRecipients
// if it matches DenyTo, or ErrRecipientsNotAllowed if it does not match
// AllowTo or its domain is not in AllowToDomains. When both allow lists are
// set, the recipient must match both.
func (f *Filter) CheckRecipient(to string) error {
	if f == nil {
		return nil
	}
	if f.DenyTo != nil && f.DenyTo.MatchString(to) {
		return ErrDeniedRecipients
	}
	if f.AllowTo != nil && !f.AllowTo.MatchString(to) {
		return ErrRecipientsNotAllowed
	}
	if len(f.AllowToDomains) > 0 && !f.domainAllowed(to) {
		return ErrRecipientsNotAllowed
	}
	return nil
}

func (f *Filter) domainAllowed(address string) bool {
	at := strings.LastIndex(address, "@")
	if at < 0 {
		return false
	}
	domain := strings.ToLower(address[at+1:])
	for _, d := range f.AllowToDomains {
		if domain == d {
			return true
		}
	}
	return false
}

// Addresses validates the sender and recipients and returns lists for allowed
// and denied recipients.
// If the sender is denied, all recipients are denied and ErrDeniedSender is
// returned. If some recipients are denied, the error of the first one is
// returned.
func (f *Filter) Addresses(
	from string,
	to []string,
) (allowedRecipients []string, deniedRecipients []string, err error) {
	allowedRecipients = []string{}
	deniedRecipients = []string{}
	if f != nil && f.AllowFrom != nil && !f.AllowFrom.MatchString(from) {
		return allowedRecipients, append(deniedRecipients, to...), ErrDeniedSender
	}
	for _, recipient := range to {
		if rcptErr := f.CheckRecipient(recipient); rcptErr != nil {
			deniedRecipients = append(deniedRecipients, recipient)
			if err == nil {
				err = rcptErr
			}
		} else {
			allowedRecipients = append(allowedRecipients, recipient)
		}
	}
	return
}

// ARNs holds Amazon Resource Names for cross-account authorization.
type ARNs struct {
	SourceArn     *string
	FromArn       *string
	ReturnPathArn *string
}

// Client provides an interface to send emails.
type Client interface {
	Send(
		origin net.Addr,
		from string,
		to []string,
		data []byte,
	) error
}

type logEntry struct {
	Time  time.Time
	IP    string
	From  string
	To    []string
	Error *string
}

// Log creates a log entry and prints it as JSON to STDOUT.
func Log(origin net.Addr, from string, to []string, err error) {
	ip := origin.(*net.TCPAddr).IP.String()
	entry := &logEntry{
		Time: time.Now().UTC(),
		IP:   ip,
		From: from,
		To:   to,
	}
	if err != nil {
		errString := err.Error()
		entry.Error = &errString
	}
	b, _ := json.Marshal(entry)
	fmt.Println(string(b))
}

// FilterAddresses validates sender and recipients against the sender allow
// list and the recipient deny list. See Filter.Addresses.
func FilterAddresses(
	from string,
	to []string,
	allowFromRegExp *regexp.Regexp,
	denyToRegExp *regexp.Regexp,
) (allowedRecipients []string, deniedRecipients []string, err error) {
	f := &Filter{AllowFrom: allowFromRegExp, DenyTo: denyToRegExp}
	return f.Addresses(from, to)
}

// RcptHandler refuses a recipient that the filter denies at RCPT. A refusal
// after DATA would come after the send to the other recipients, and the
// client would send the message to them again.
func RcptHandler(f *Filter) func(origin net.Addr, from string, to string) bool {
	return func(origin net.Addr, from string, to string) bool {
		if err := f.CheckRecipient(to); err != nil {
			Log(origin, from, []string{to}, err)
			return false
		}
		return true
	}
}
