package relay

import (
	"context"
	"net"
	"regexp"

	"github.com/KamorionLabs/aws-smtp-relay/internal/relay"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	sestypes "github.com/aws/aws-sdk-go-v2/service/ses/types"
)

// SESEmailClient interface for testing
type SESEmailClient interface {
	SendRawEmail(context.Context, *ses.SendRawEmailInput, ...func(*ses.Options)) (*ses.SendRawEmailOutput, error)
}

// Client implements the Relay interface.
type Client struct {
	sesClient       SESEmailClient
	setName         *string
	allowFromRegExp *regexp.Regexp
	denyToRegExp    *regexp.Regexp
	arns            *relay.ARNs
}

// Send uses the client SESEmailClient to send email data
func (c Client) Send(
	origin net.Addr,
	from string,
	to []string,
	data []byte,
) error {
	allowedRecipients, deniedRecipients, err := relay.FilterAddresses(
		from,
		to,
		c.allowFromRegExp,
		c.denyToRegExp,
	)
	if err != nil {
		relay.Log(origin, from, deniedRecipients, err)
	}
	if len(allowedRecipients) > 0 {
		input := &ses.SendRawEmailInput{
			ConfigurationSetName: c.setName,
			Source:               &from,
			Destinations:         allowedRecipients,
			RawMessage:           &sestypes.RawMessage{Data: data},
		}
		if c.arns != nil {
			input.SourceArn = c.arns.SourceArn
			input.FromArn = c.arns.FromArn
			input.ReturnPathArn = c.arns.ReturnPathArn
		}
		_, err := c.sesClient.SendRawEmail(context.Background(), input)
		relay.Log(origin, from, allowedRecipients, err)
		if err != nil {
			return err
		}
	}
	return err
}

// New creates a new client with AWS SDK v2 configuration.
func New(
	configurationSetName *string,
	allowFromRegExp *regexp.Regexp,
	denyToRegExp *regexp.Regexp,
	arns *relay.ARNs,
) Client {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic("unable to load SDK config, " + err.Error())
	}
	return Client{
		sesClient:       ses.NewFromConfig(cfg),
		setName:         configurationSetName,
		allowFromRegExp: allowFromRegExp,
		denyToRegExp:    denyToRegExp,
		arns:            arns,
	}
}
