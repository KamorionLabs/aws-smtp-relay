package relay

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"regexp"
	"testing"

	"github.com/KamorionLabs/aws-smtp-relay/internal/relay"
	"github.com/aws/aws-sdk-go-v2/service/ses"
)

var testData = struct {
	input *ses.SendRawEmailInput
	err   error
}{}

type mockSESClient struct{}

func (m *mockSESClient) SendRawEmail(ctx context.Context, input *ses.SendRawEmailInput, opts ...func(*ses.Options)) (
	*ses.SendRawEmailOutput,
	error,
) {
	testData.input = input
	return nil, testData.err
}

func sendHelper(
	origin net.Addr,
	from string,
	to []string,
	data []byte,
	configurationSetName *string,
	allowFromRegExp *regexp.Regexp,
	denyToRegExp *regexp.Regexp,
	arns *relay.ARNs,
	apiErr error,
) (email *ses.SendRawEmailInput, out []byte, err []byte, sendErr error) {
	outReader, outWriter, _ := os.Pipe()
	errReader, errWriter, _ := os.Pipe()
	originalOut := os.Stdout
	originalErr := os.Stderr
	defer func() {
		testData.input = nil
		testData.err = nil
		os.Stdout = originalOut
		os.Stderr = originalErr
	}()
	os.Stdout = outWriter
	os.Stderr = errWriter
	func() {
		c := Client{
			sesClient:       &mockSESClient{},
			setName:         configurationSetName,
			allowFromRegExp: allowFromRegExp,
			denyToRegExp:    denyToRegExp,
			arns:            arns,
		}
		testData.err = apiErr
		sendErr = c.Send(origin, from, to, data)
		outWriter.Close()
		errWriter.Close()
	}()
	stdout, _ := io.ReadAll(outReader)
	stderr, _ := io.ReadAll(errReader)
	return testData.input, stdout, stderr, sendErr
}

func TestSend(t *testing.T) {
	origin := net.TCPAddr{IP: []byte{127, 0, 0, 1}}
	from := "alice@example.org"
	to := []string{"bob@example.org"}
	data := []byte{'T', 'E', 'S', 'T'}
	setName := ""
	input, out, err, _ := sendHelper(&origin, from, to, data, &setName, nil, nil, nil, nil)
	if *input.Source != from {
		t.Errorf(
			"Unexpected source: %s. Expected: %s",
			*input.Source,
			from,
		)
	}
	if len(input.Destinations) != 1 {
		t.Errorf(
			"Unexpected number of destinations: %d. Expected: %d",
			len(input.Destinations),
			1,
		)
	}
	if input.Destinations[0] != to[0] {
		t.Errorf(
			"Unexpected destination: %s. Expected: %s",
			input.Destinations[0],
			to[0],
		)
	}
	inputData := string(input.RawMessage.Data)
	if inputData != "TEST" {
		t.Errorf("Unexpected data: %s. Expected: %s", inputData, "TEST")
	}
	if len(out) == 0 {
		t.Error("Unexpected empty stdout")
	}
	if len(err) != 0 {
		t.Errorf("Unexpected stderr: %s", err)
	}
}

func TestSendWithMultipleRecipients(t *testing.T) {
	origin := net.TCPAddr{IP: []byte{127, 0, 0, 1}}
	from := "alice@example.org"
	to := []string{"bob@example.org", "charlie@example.org"}
	data := []byte{'T', 'E', 'S', 'T'}
	setName := ""
	input, out, err, _ := sendHelper(&origin, from, to, data, &setName, nil, nil, nil, nil)
	if len(input.Destinations) != 2 {
		t.Errorf(
			"Unexpected number of destinations: %d. Expected: %d",
			len(input.Destinations),
			2,
		)
	}
	if input.Destinations[0] != to[0] {
		t.Errorf(
			"Unexpected destination: %s. Expected: %s",
			input.Destinations[0],
			to[0],
		)
	}
	if len(out) == 0 {
		t.Error("Unexpected empty stdout")
	}
	if len(err) != 0 {
		t.Errorf("Unexpected stderr: %s", err)
	}
}

func TestSendWithDeniedSender(t *testing.T) {
	origin := net.TCPAddr{IP: []byte{127, 0, 0, 1}}
	from := "alice@example.org"
	to := []string{"bob@example.org", "charlie@example.org"}
	data := []byte{'T', 'E', 'S', 'T'}
	setName := ""
	regexp, _ := regexp.Compile(`^admin@example\.org$`)
	input, out, err, sendErr := sendHelper(&origin, from, to, data, &setName, regexp, nil, nil, nil)
	if input != nil {
		t.Errorf(
			"Unexpected number of destinations: %d. Expected: %d",
			len(input.Destinations),
			0,
		)
	}
	if sendErr != relay.ErrDeniedSender {
		t.Errorf("Unexpected error: %s. Expected: %s", sendErr, relay.ErrDeniedSender)
	}
	if len(out) == 0 {
		t.Error("Unexpected empty stdout")
	}
	if len(err) != 0 {
		t.Errorf("Unexpected stderr: %s", err)
	}
}

func TestSendWithDeniedRecipient(t *testing.T) {
	origin := net.TCPAddr{IP: []byte{127, 0, 0, 1}}
	from := "alice@example.org"
	to := []string{"bob@example.org", "charlie@example.org"}
	data := []byte{'T', 'E', 'S', 'T'}
	setName := ""
	regexp, _ := regexp.Compile(`^bob@example\.org$`)
	input, out, err, sendErr := sendHelper(&origin, from, to, data, &setName, nil, regexp, nil, nil)
	if len(input.Destinations) != 1 {
		t.Errorf(
			"Unexpected number of destinations: %d. Expected: %d",
			len(input.Destinations),
			1,
		)
	}
	if input.Destinations[0] != to[1] {
		t.Errorf(
			"Unexpected destination: %s. Expected: %s",
			input.Destinations[0],
			to[1],
		)
	}
	if sendErr != relay.ErrDeniedRecipients {
		t.Errorf("Unexpected error: %s. Expected: %s", sendErr, relay.ErrDeniedRecipients)
	}
	if len(out) == 0 {
		t.Error("Unexpected empty stdout")
	}
	if len(err) != 0 {
		t.Errorf("Unexpected stderr: %s", err)
	}
}

func TestSendWithApiError(t *testing.T) {
	origin := net.TCPAddr{IP: []byte{127, 0, 0, 1}}
	from := "alice@example.org"
	to := []string{"bob@example.org"}
	data := []byte{'T', 'E', 'S', 'T'}
	setName := ""
	apiErr := errors.New("API failure")
	input, out, err, sendErr := sendHelper(&origin, from, to, data, &setName, nil, nil, nil, apiErr)
	if *input.Source != from {
		t.Errorf(
			"Unexpected source: %s. Expected: %s",
			*input.Source,
			from,
		)
	}
	if len(input.Destinations) != 1 {
		t.Errorf(
			"Unexpected number of destinations: %d. Expected: %d",
			len(input.Destinations),
			1,
		)
	}
	if input.Destinations[0] != to[0] {
		t.Errorf(
			"Unexpected destination: %s. Expected: %s",
			input.Destinations[0],
			to[0],
		)
	}
	inputData := string(input.RawMessage.Data)
	if inputData != "TEST" {
		t.Errorf("Unexpected data: %s. Expected: %s", inputData, "TEST")
	}
	if sendErr != apiErr {
		t.Errorf("Send did not report API error: %s. Expected: %s", sendErr, apiErr)
	}
	if len(out) == 0 {
		t.Error("Unexpected empty stdout")
	}
	if len(err) != 0 {
		t.Errorf("Unexpected stderr: %s", err)
	}
}

func TestNew(t *testing.T) {
	setName := ""
	allowFromRegExp, _ := regexp.Compile(`^admin@example\.org$`)
	denyToRegExp, _ := regexp.Compile(`^bob@example\.org$`)
	sourceArn := "arn:aws:ses:us-east-1:123456789012:identity/example.com"
	fromArn := "arn:aws:ses:us-east-1:123456789012:identity/from.example.com"
	returnPathArn := "arn:aws:ses:us-east-1:123456789012:identity/return.example.com"
	arns := &relay.ARNs{
		SourceArn:     &sourceArn,
		FromArn:       &fromArn,
		ReturnPathArn: &returnPathArn,
	}
	client := New(&setName, allowFromRegExp, denyToRegExp, arns)
	_, ok := interface{}(client).(relay.Client)
	if !ok {
		t.Error("Unexpected: client is not a relay.Client")
	}
	if client.setName != &setName {
		t.Errorf("Unexpected setName: %s", *client.setName)
	}
	if client.allowFromRegExp != allowFromRegExp {
		t.Errorf("Unexpected allowFromRegExp: %s", client.allowFromRegExp)
	}
	if client.denyToRegExp != denyToRegExp {
		t.Errorf("Unexpected denyToRegExp: %s", client.denyToRegExp)
	}
	if client.arns != arns {
		t.Errorf("Unexpected arns: %v", client.arns)
	}
	if client.arns.SourceArn != &sourceArn {
		t.Errorf("Unexpected sourceArn: %s", *client.arns.SourceArn)
	}
	if client.arns.FromArn != &fromArn {
		t.Errorf("Unexpected fromArn: %s", *client.arns.FromArn)
	}
	if client.arns.ReturnPathArn != &returnPathArn {
		t.Errorf("Unexpected returnPathArn: %s", *client.arns.ReturnPathArn)
	}
}
