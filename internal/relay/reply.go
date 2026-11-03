package relay

import (
	"errors"
	"strings"
	"unicode"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/smithy-go"
)

// replyError is an error with an SMTP reply. mhale/smtpd sends the text of a
// handler error that starts with a reply code as the reply.
type replyError struct {
	reply string
	err   error
}

func (e *replyError) Error() string {
	return e.reply
}

func (e *replyError) Unwrap() error {
	return e.err
}

var retryables = retry.IsErrorRetryables(retry.DefaultRetryables)

// setupErrorCodes are errors of the account, not of the message. The same
// message goes through after an operator fixes them.
var setupErrorCodes = map[string]bool{
	"SendingPausedException":             true,
	"AccountSuspendedException":          true,
	"MailFromDomainNotVerifiedException": true,
}

// isTemporary reports whether a send that failed with err can succeed later:
// a cancelled or timed-out call, an error the SDK retryer retries, an error of
// the setup (401, 403, setupErrorCodes), a 429 or 5xx response, or no
// response at all. Only a fault of the message itself is permanent.
func isTemporary(err error) bool {
	var canceled *smithy.CanceledError
	if errors.As(err, &canceled) {
		return true
	}
	if v := retryables.IsErrorRetryable(err); v != aws.UnknownTernary {
		return v == aws.TrueTernary
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && setupErrorCodes[apiErr.ErrorCode()] {
		return true
	}
	var respErr *awshttp.ResponseError
	if errors.As(err, &respErr) {
		code := respErr.HTTPStatusCode()
		return code == 401 || code == 403 || code == 429 || code >= 500
	}
	return true
}

// Reply turns an error of Client.Send into an error with an SMTP reply code,
// so that the client tries again only when that can help.
func Reply(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrDeniedSender):
		return &replyError{"550 5.7.1 Sender address not allowed", err}
	case errors.Is(err, ErrDeniedRecipients):
		return &replyError{"550 5.7.1 Recipient address not allowed", err}
	case isTemporary(err):
		return &replyError{"451 4.3.0 Temporary error, try again later" + errorCode(err), err}
	default:
		return &replyError{"554 5.0.0 Message refused" + errorCode(err), err}
	}
}

// errorCode returns ": <API error code>", or "" when err has none. The full
// error text can have AWS account details and line breaks; it goes to the log.
func errorCode(err error) string {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return ""
	}
	code := strings.Map(func(r rune) rune {
		if r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_') {
			return r
		}
		return -1
	}, apiErr.ErrorCode())
	if code == "" {
		return ""
	}
	return ": " + code
}
