package relay

import (
	"context"
	"net"
	"regexp"

	"github.com/KamorionLabs/aws-smtp-relay/internal/relay"
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
	pinpointClient  PinpointEmailClient
	setName         *string
	allowFromRegExp *regexp.Regexp
	denyToRegExp    *regexp.Regexp
}

// Send uses the given Pinpoint API to send email data
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
		_, err := c.pinpointClient.SendEmail(context.Background(), &pinpointemail.SendEmailInput{
			ConfigurationSetName: c.setName,
			FromEmailAddress:     &from,
			Destination: &pinpointemailtypes.Destination{
				ToAddresses: allowedRecipients,
			},
			Content: &pinpointemailtypes.EmailContent{
				Raw: &pinpointemailtypes.RawMessage{
					Data: data,
				},
			},
		})
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
) Client {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic("unable to load SDK config, " + err.Error())
	}
	return Client{
		pinpointClient:  pinpointemail.NewFromConfig(cfg),
		setName:         configurationSetName,
		allowFromRegExp: allowFromRegExp,
		denyToRegExp:    denyToRegExp,
	}
}
