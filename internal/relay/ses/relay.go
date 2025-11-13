package ses

import (
	"context"
	"io"
	"net"
	"regexp"

	"github.com/KamorionLabs/aws-smtp-relay/internal"
	"github.com/KamorionLabs/aws-smtp-relay/internal/relay"
	"github.com/KamorionLabs/aws-smtp-relay/internal/relay/filter"
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
	SesClient       SESEmailClient
	setName         *string
	allowFromRegExp *regexp.Regexp
	denyToRegExp    *regexp.Regexp
	allowToRegExp   *regexp.Regexp
	allowedDomains  []string
	maxMessageSize  uint
	arns            *relay.ARNs
}

func (c Client) Annotate(_clt relay.Client) relay.Client {
	clt := _clt.(*Client)
	pclt := c.SesClient
	if clt.SesClient != nil {
		pclt = clt.SesClient
	}
	return &Client{
		SesClient:       pclt,
		setName:         c.setName,
		allowFromRegExp: c.allowFromRegExp,
		denyToRegExp:    c.denyToRegExp,
		allowToRegExp:   c.allowToRegExp,
		allowedDomains:  c.allowedDomains,
		maxMessageSize:  c.maxMessageSize,
		arns:            c.arns,
	}
}

// Send uses the client SESEmailClient to send email data via SESv2 API
func (c Client) Send(
	origin net.Addr,
	from string,
	to []string,
	dr io.Reader,
) error {
	allowedRecipients, deniedRecipients, err := filter.FilterAddresses(
		from,
		to,
		c.allowFromRegExp,
		c.denyToRegExp,
		c.allowToRegExp,
		c.allowedDomains,
	)
	if err != nil {
		internal.Log(origin, from, deniedRecipients, err)
	}

	if len(allowedRecipients) > 0 {
		data, sendErr := relay.ConsumeToBytes(dr, c.maxMessageSize)
		if sendErr != nil {
			return sendErr
		}

		input := &sesv2.SendEmailInput{
			ConfigurationSetName: c.setName,
			FromEmailAddress:     &from,
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

		_, sendErr = c.SesClient.SendEmail(context.Background(), input)
		if sendErr != nil {
			err = sendErr
		}
		internal.Log(origin, from, allowedRecipients, err)
	}
	return err
}

// New creates a new client with AWS SDK v2 configuration using SESv2 API.
func New(
	configurationSetName *string,
	allowFromRegExp *regexp.Regexp,
	denyToRegExp *regexp.Regexp,
	allowToRegExp *regexp.Regexp,
	allowedDomains []string,
	maxMessageSize uint,
	arns *relay.ARNs,
) Client {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic("unable to load SDK config, " + err.Error())
	}
	return Client{
		SesClient:       sesv2.NewFromConfig(cfg),
		setName:         configurationSetName,
		allowFromRegExp: allowFromRegExp,
		denyToRegExp:    denyToRegExp,
		allowToRegExp:   allowToRegExp,
		allowedDomains:  allowedDomains,
		maxMessageSize:  maxMessageSize,
		arns:            arns,
	}
}
