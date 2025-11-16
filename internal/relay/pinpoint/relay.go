package pinpoint

import (
	"context"
	"io"
	"net"
	"regexp"

	"github.com/KamorionLabs/aws-smtp-relay/internal"
	"github.com/KamorionLabs/aws-smtp-relay/internal/relay"
	"github.com/KamorionLabs/aws-smtp-relay/internal/relay/filter"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/pinpointemail"
	pinpointemailtypes "github.com/aws/aws-sdk-go-v2/service/pinpointemail/types"
)

// PinpointEmailClient interface for testing
type PinpointEmailClient interface {
	SendEmail(context.Context, *pinpointemail.SendEmailInput, ...func(*pinpointemail.Options)) (*pinpointemail.SendEmailOutput, error)
}

// Client implements the Relay interface.
type Client struct {
	PinpointClient  PinpointEmailClient
	setName         *string
	allowFromRegExp *regexp.Regexp
	denyToRegExp    *regexp.Regexp
	allowToRegExp   *regexp.Regexp
	allowedDomains  []string
	maxMessageSize  uint
}

func (c Client) Annotate(rclt relay.Client) relay.Client {
	clt := rclt.(*Client)
	pclt := c.PinpointClient
	if clt.PinpointClient != nil {
		pclt = clt.PinpointClient
	}
	return &Client{
		PinpointClient:  pclt,
		setName:         c.setName,
		allowFromRegExp: c.allowFromRegExp,
		denyToRegExp:    c.denyToRegExp,
		allowToRegExp:   c.allowToRegExp,
		allowedDomains:  c.allowedDomains,
		maxMessageSize:  c.maxMessageSize,
	}
}

// Send uses the given Pinpoint API to send email data
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
		_, sendErr = c.PinpointClient.SendEmail(context.Background(), &pinpointemail.SendEmailInput{
			Content:                        &pinpointemailtypes.EmailContent{Raw: &pinpointemailtypes.RawMessage{Data: data}},
			Destination:                    &pinpointemailtypes.Destination{ToAddresses: allowedRecipients},
			ConfigurationSetName:           c.setName,
			EmailTags:                      []pinpointemailtypes.MessageTag{},
			FeedbackForwardingEmailAddress: new(string),
			FromEmailAddress:               &from,
			ReplyToAddresses:               to,
		})
		if sendErr != nil {
			err = sendErr
		}
		internal.Log(origin, from, allowedRecipients, err)
	}
	return err
}

// New creates a new client with AWS SDK v2 configuration.
func New(
	configurationSetName *string,
	allowFromRegExp *regexp.Regexp,
	denyToRegExp *regexp.Regexp,
	allowToRegExp *regexp.Regexp,
	allowedDomains []string,
	maxMessageSize uint,
) Client {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic("unable to load SDK config, " + err.Error())
	}
	return Client{
		PinpointClient:  pinpointemail.NewFromConfig(cfg),
		setName:         configurationSetName,
		allowFromRegExp: allowFromRegExp,
		denyToRegExp:    denyToRegExp,
		allowToRegExp:   allowToRegExp,
		allowedDomains:  allowedDomains,
		maxMessageSize:  maxMessageSize,
	}
}
