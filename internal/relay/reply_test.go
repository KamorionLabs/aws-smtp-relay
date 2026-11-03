package relay

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"testing"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

func responseError(status int, err error) error {
	return &awshttp.ResponseError{ResponseError: &smithyhttp.ResponseError{
		Response: &smithyhttp.Response{Response: &http.Response{StatusCode: status}},
		Err:      err,
	}}
}

func TestReply(t *testing.T) {
	notVerified := "Email address is not verified."
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"denied sender", ErrDeniedSender, "550 5.7.1 Sender address not allowed"},
		{"denied recipients", ErrDeniedRecipients, "550 5.7.1 Recipient address not allowed"},
		{"throttled", responseError(429, &sesv2types.TooManyRequestsException{}),
			"451 4.3.0 Temporary error, try again later: TooManyRequestsException"},
		{"throttling code on 400", responseError(400, &smithy.GenericAPIError{Code: "Throttling"}),
			"451 4.3.0 Temporary error, try again later: Throttling"},
		{"server error", responseError(500, &smithy.GenericAPIError{Code: "InternalFailure"}),
			"451 4.3.0 Temporary error, try again later: InternalFailure"},
		{"network", &net.OpError{Op: "dial", Err: errors.New("refused")},
			"451 4.3.0 Temporary error, try again later"},
		{"timeout", context.DeadlineExceeded, "451 4.3.0 Temporary error, try again later"},
		{"cancelled call", &smithy.CanceledError{Err: context.DeadlineExceeded},
			"451 4.3.0 Temporary error, try again later"},
		{"rejected", responseError(400, &sesv2types.MessageRejected{Message: &notVerified}),
			"554 5.0.0 Message refused: MessageRejected"},
		{"bad request", responseError(400, &sesv2types.BadRequestException{}),
			"554 5.0.0 Message refused: BadRequestException"},
		{"mail from", responseError(400, &sesv2types.MailFromDomainNotVerifiedException{}),
			"451 4.3.0 Temporary error, try again later: MailFromDomainNotVerifiedException"},
		{"sending paused", responseError(400, &sesv2types.SendingPausedException{}),
			"451 4.3.0 Temporary error, try again later: SendingPausedException"},
		{"account suspended", responseError(400, &sesv2types.AccountSuspendedException{}),
			"451 4.3.0 Temporary error, try again later: AccountSuspendedException"},
		{"expired token", responseError(401, &smithy.GenericAPIError{Code: "ExpiredTokenException"}),
			"451 4.3.0 Temporary error, try again later: ExpiredTokenException"},
		// Only the code goes into the reply: no account details, no line breaks.
		{"access denied", responseError(403, &smithy.GenericAPIError{
			Code:    "AccessDeniedException",
			Message: "User: arn:aws:sts::123456789012:assumed-role/x is not authorized\r\n100%",
		}), "451 4.3.0 Temporary error, try again later: AccessDeniedException"},
		{"odd code", responseError(400, &smithy.GenericAPIError{Code: "Bad%s\r\nCode"}),
			"554 5.0.0 Message refused: BadsCode"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Reply(fmt.Errorf("send: %w", tc.err))
			if got := err.Error(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
			if !errors.Is(err, tc.err) {
				t.Error("the reply does not wrap the original error")
			}
		})
	}
	if err := Reply(nil); err != nil {
		t.Errorf("Reply(nil) = %v, want nil", err)
	}
}

func TestRcptHandler(t *testing.T) {
	origin := &net.TCPAddr{IP: net.ParseIP("127.0.0.1")}
	accept := RcptHandler(regexp.MustCompile(`^bob@`))
	if !accept(origin, "alice@example.org", "carol@example.org") {
		t.Error("an allowed recipient was refused")
	}
	if accept(origin, "alice@example.org", "bob@example.org") {
		t.Error("a denied recipient was accepted")
	}
	if !RcptHandler(nil)(origin, "alice@example.org", "bob@example.org") {
		t.Error("a recipient was refused without a deny regexp")
	}
}
