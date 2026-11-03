package relay

import (
	"context"
	"net"
	"regexp"

	"github.com/KamorionLabs/aws-smtp-relay/internal/relay"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

// SESEmailClient interface for testing
type SESEmailClient interface {
	SendEmail(context.Context, *sesv2.SendEmailInput, ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
}

// Client implements the Relay interface.
type Client struct {
	sesClient       SESEmailClient
	setName         *string
	allowFromRegExp *regexp.Regexp
	denyToRegExp    *regexp.Regexp
	arns            *relay.ARNs
}

// Send uses the client SESEmailClient to send email data via SESv2 API
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
		// Avoid setting FromEmailAddress to let SES extract it from raw data.
		// If set, it should include the friendly name (if one is used) and source email.
		input := &sesv2.SendEmailInput{
			ConfigurationSetName: c.setName,
			Destination: &sesv2types.Destination{
				ToAddresses: allowedRecipients,
			},
			Content: &sesv2types.EmailContent{
				Raw: &sesv2types.RawMessage{
					Data: data,
				},
			},
		}
		// Map ARNs to SESv2 format
		// FromArn and SourceArn both map to FromEmailAddressIdentityArn
		if c.arns != nil {
			if c.arns.FromArn != nil {
				input.FromEmailAddressIdentityArn = c.arns.FromArn
			} else if c.arns.SourceArn != nil {
				input.FromEmailAddressIdentityArn = c.arns.SourceArn
			}
			// ReturnPathArn maps to FeedbackForwardingEmailAddressIdentityArn
			if c.arns.ReturnPathArn != nil {
				input.FeedbackForwardingEmailAddressIdentityArn = c.arns.ReturnPathArn
			}
		}
		_, err := c.sesClient.SendEmail(context.Background(), input)
		relay.Log(origin, from, allowedRecipients, err)
		if err != nil {
			return err
		}
	}
	return err
}

// New creates a new client with AWS SDK v2 configuration using SESv2 API.
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
		sesClient:       sesv2.NewFromConfig(cfg),
		setName:         configurationSetName,
		allowFromRegExp: allowFromRegExp,
		denyToRegExp:    denyToRegExp,
		arns:            arns,
	}
}
