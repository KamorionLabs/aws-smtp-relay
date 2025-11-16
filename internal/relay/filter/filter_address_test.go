package filter

import (
	"regexp"
	"testing"
)

func TestFilterAddresses_AllowTo(t *testing.T) {
	allowToRegExp, _ := regexp.Compile(`^allowed@example\.org$`)

	allowed, denied, err := FilterAddresses(
		"sender@example.com",
		[]string{"allowed@example.org", "other@example.org"},
		nil,
		nil,
		allowToRegExp,
		[]string{},
	)

	if len(allowed) != 1 {
		t.Errorf("Expected 1 allowed recipient, got %d", len(allowed))
	}
	if len(denied) != 1 {
		t.Errorf("Expected 1 denied recipient, got %d", len(denied))
	}
	if allowed[0] != "allowed@example.org" {
		t.Errorf("Expected allowed@example.org in allowed list, got %s", allowed[0])
	}
	if denied[0] != "other@example.org" {
		t.Errorf("Expected other@example.org in denied list, got %s", denied[0])
	}
	if err != ErrDeniedRecipientsNotAllowed {
		t.Errorf("Expected ErrDeniedRecipientsNotAllowed, got %v", err)
	}
}

func TestFilterAddresses_AllowToDomains(t *testing.T) {
	allowed, denied, err := FilterAddresses(
		"sender@example.com",
		[]string{"user@example.org", "user@other.com", "admin@example.org"},
		nil,
		nil,
		nil,
		[]string{"example.org"},
	)

	if len(allowed) != 2 {
		t.Errorf("Expected 2 allowed recipients, got %d", len(allowed))
	}
	if len(denied) != 1 {
		t.Errorf("Expected 1 denied recipient, got %d", len(denied))
	}
	if denied[0] != "user@other.com" {
		t.Errorf("Expected user@other.com in denied list, got %s", denied[0])
	}
	if err != ErrDeniedRecipientsNotAllowed {
		t.Errorf("Expected ErrDeniedRecipientsNotAllowed, got %v", err)
	}
}

func TestFilterAddresses_MultipleDomains(t *testing.T) {
	allowed, denied, err := FilterAddresses(
		"sender@example.com",
		[]string{"user@example.org", "user@example.com", "admin@other.org"},
		nil,
		nil,
		nil,
		[]string{"example.org", "example.com"},
	)

	if len(allowed) != 2 {
		t.Errorf("Expected 2 allowed recipients, got %d", len(allowed))
	}
	if len(denied) != 1 {
		t.Errorf("Expected 1 denied recipient, got %d", len(denied))
	}
	if err != ErrDeniedRecipientsNotAllowed {
		t.Errorf("Expected ErrDeniedRecipientsNotAllowed, got %v", err)
	}
}

func TestFilterAddresses_CombinedFilters(t *testing.T) {
	allowToRegExp, _ := regexp.Compile(`^admin@`)
	denyToRegExp, _ := regexp.Compile(`^admin@denied\.org$`)

	allowed, denied, err := FilterAddresses(
		"sender@example.com",
		[]string{"admin@example.org", "admin@denied.org", "user@example.org"},
		nil,
		denyToRegExp,
		allowToRegExp,
		[]string{"example.org"},
	)

	// admin@example.org should be allowed (matches allowTo, not in denyTo, in allowed domain)
	// admin@denied.org should be denied (matches allowTo but in denyTo)
	// user@example.org should be denied (doesn't match allowTo)
	if len(allowed) != 1 {
		t.Errorf("Expected 1 allowed recipient, got %d", len(allowed))
	}
	if len(denied) != 2 {
		t.Errorf("Expected 2 denied recipients, got %d", len(denied))
	}
	if allowed[0] != "admin@example.org" {
		t.Errorf("Expected admin@example.org in allowed list, got %s", allowed[0])
	}
	if err != ErrDeniedRecipientsNotAllowed {
		t.Errorf("Expected ErrDeniedRecipientsNotAllowed, got %v", err)
	}
}

func TestFilterAddresses_AllowFromDenied(t *testing.T) {
	allowFromRegExp, _ := regexp.Compile(`^allowed@sender\.com$`)
	allowToRegExp, _ := regexp.Compile(`^admin@`)

	allowed, denied, err := FilterAddresses(
		"notallowed@sender.com",
		[]string{"admin@example.org", "user@example.org"},
		allowFromRegExp,
		nil,
		allowToRegExp,
		[]string{},
	)

	// All recipients should be denied because sender is denied
	if len(allowed) != 0 {
		t.Errorf("Expected 0 allowed recipients, got %d", len(allowed))
	}
	if len(denied) != 2 {
		t.Errorf("Expected 2 denied recipients, got %d", len(denied))
	}
	if err != ErrDeniedSender {
		t.Errorf("Expected ErrDeniedSender, got %v", err)
	}
}

func TestFilterAddresses_NoFilters(t *testing.T) {
	allowed, denied, err := FilterAddresses(
		"sender@example.com",
		[]string{"user1@example.org", "user2@example.org"},
		nil,
		nil,
		nil,
		[]string{},
	)

	// Without any filters, all recipients should be allowed
	if len(allowed) != 2 {
		t.Errorf("Expected 2 allowed recipients, got %d", len(allowed))
	}
	if len(denied) != 0 {
		t.Errorf("Expected 0 denied recipients, got %d", len(denied))
	}
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestFilterAddresses_DenyToOnly(t *testing.T) {
	denyToRegExp, _ := regexp.Compile(`^blocked@`)

	allowed, denied, err := FilterAddresses(
		"sender@example.com",
		[]string{"blocked@example.org", "allowed@example.org"},
		nil,
		denyToRegExp,
		nil,
		[]string{},
	)

	// Only recipients matching denyTo should be denied
	if len(allowed) != 1 {
		t.Errorf("Expected 1 allowed recipient, got %d", len(allowed))
	}
	if len(denied) != 1 {
		t.Errorf("Expected 1 denied recipient, got %d", len(denied))
	}
	if denied[0] != "blocked@example.org" {
		t.Errorf("Expected blocked@example.org in denied list, got %s", denied[0])
	}
	if err != ErrDeniedRecipients {
		t.Errorf("Expected ErrDeniedRecipients, got %v", err)
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		email    string
		expected string
	}{
		{"user@example.org", "example.org"},
		{"admin@mail.example.com", "mail.example.com"},
		{"invalid-email", ""},
		{"", ""},
	}

	for _, test := range tests {
		result := extractDomain(test.email)
		if result != test.expected {
			t.Errorf("extractDomain(%s) = %s, expected %s", test.email, result, test.expected)
		}
	}
}

func TestIsAllowedDomain(t *testing.T) {
	tests := []struct {
		email          string
		allowedDomains []string
		expected       bool
	}{
		{"user@example.org", []string{"example.org"}, true},
		{"user@example.org", []string{"example.org", "other.com"}, true},
		{"user@other.com", []string{"example.org"}, false},
		{"user@example.org", []string{}, true}, // Empty list allows all
		{"invalid-email", []string{"example.org"}, false},
	}

	for _, test := range tests {
		result := isAllowedDomain(test.email, test.allowedDomains)
		if result != test.expected {
			t.Errorf("isAllowedDomain(%s, %v) = %v, expected %v",
				test.email, test.allowedDomains, result, test.expected)
		}
	}
}
