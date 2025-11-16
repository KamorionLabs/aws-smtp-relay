package filter

import (
	"errors"
	"regexp"
)

var (
	ErrDeniedSender = errors.New(
		"denied sender: sender does not match the allowed emails regexp",
	)

	ErrDeniedRecipients = errors.New(
		"denied recipients: recipients match the denied emails regexp",
	)

	ErrDeniedRecipientsNotAllowed = errors.New(
		"denied recipients: recipients do not match the allowed emails regexp or domains",
	)
)

// extractDomain extracts the domain from an email address
func extractDomain(email string) string {
	parts := regexp.MustCompile("@").Split(email, 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

// isAllowedDomain checks if the email's domain is in the allowed domains list
func isAllowedDomain(email string, allowedDomains []string) bool {
	if len(allowedDomains) == 0 {
		return true // No restriction if list is empty
	}
	domain := extractDomain(email)
	for _, allowedDomain := range allowedDomains {
		if domain == allowedDomain {
			return true
		}
	}
	return false
}

// FilterAddresses validates sender and recipients and returns lists for allowed
// and denied recipients.
// If the sender is denied, all recipients are denied and an error is returned.
// If the sender is allowed, but some of the recipients are denied, an error
// will also be returned.
//
// Filtering logic:
// - allowFromRegExp: Whitelist for sender addresses (if set, sender must match)
// - denyToRegExp: Blacklist for recipient addresses (if set, recipients matching are denied)
// - allowToRegExp: Whitelist for recipient addresses (if set, recipients must match)
// - allowedDomains: Whitelist for recipient domains (if set, recipients must be in these domains)
func FilterAddresses(
	from string,
	to []string,
	allowFromRegExp *regexp.Regexp,
	denyToRegExp *regexp.Regexp,
	allowToRegExp *regexp.Regexp,
	allowedDomains []string,
) (allowedRecipients []string, deniedRecipients []string, err error) {
	allowedRecipients = []string{}
	deniedRecipients = []string{}

	// Check sender against allowFrom whitelist
	if allowFromRegExp != nil && !allowFromRegExp.MatchString(from) {
		err = ErrDeniedSender
	}

	for k := range to {
		recipient := &(to)[k]

		// Deny all recipients if the sender address is not allowed
		if err != nil {
			deniedRecipients = append(deniedRecipients, *recipient)
			continue
		}

		// Check recipient against denyTo blacklist
		if denyToRegExp != nil && denyToRegExp.MatchString(*recipient) {
			deniedRecipients = append(deniedRecipients, *recipient)
			continue
		}

		// Check recipient against allowTo whitelist (if set, must match)
		if allowToRegExp != nil && !allowToRegExp.MatchString(*recipient) {
			deniedRecipients = append(deniedRecipients, *recipient)
			continue
		}

		// Check recipient domain against allowed domains (if set, must be in list)
		if !isAllowedDomain(*recipient, allowedDomains) {
			deniedRecipients = append(deniedRecipients, *recipient)
			continue
		}

		// Recipient passed all checks
		allowedRecipients = append(allowedRecipients, *recipient)
	}

	// Set appropriate error if recipients were denied
	if err == nil && len(deniedRecipients) > 0 {
		// Determine which error is most appropriate
		if allowToRegExp != nil || len(allowedDomains) > 0 {
			err = ErrDeniedRecipientsNotAllowed
		} else {
			err = ErrDeniedRecipients
		}
	}
	return
}
