/*
Package relay provides an interface to relay emails via Amazon SES/Pinpoint API.
*/
package factory

import (
	"errors"

	"github.com/KamorionLabs/aws-smtp-relay/internal/relay"
	"github.com/KamorionLabs/aws-smtp-relay/internal/relay/config"
	pinpointrelay "github.com/KamorionLabs/aws-smtp-relay/internal/relay/pinpoint"
	sesrelay "github.com/KamorionLabs/aws-smtp-relay/internal/relay/ses"
)

// toStringPtr returns nil for empty strings, otherwise returns a pointer to the string
func toStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func NewClient(cfg *config.Config) (relay.Client, error) {
	var client relay.Client
	switch cfg.RelayAPI {
	case "pinpoint":
		client = pinpointrelay.New(&cfg.SetName, cfg.AllowFromRegExp, cfg.DenyToRegExp, cfg.AllowToRegExp, cfg.AllowToDomainsSlice, uint(cfg.MaxMessageBytes))
	case "ses":
		var arns *relay.ARNs
		// Configure ARNs for cross-account authorization if any ARN is provided
		if cfg.SourceArn != "" || cfg.FromArn != "" || cfg.ReturnPathArn != "" {
			arns = &relay.ARNs{
				SourceArn:     toStringPtr(cfg.SourceArn),
				FromArn:       toStringPtr(cfg.FromArn),
				ReturnPathArn: toStringPtr(cfg.ReturnPathArn),
			}
			// If SourceArn is provided, use it as default for FromArn and ReturnPathArn
			if cfg.SourceArn != "" {
				if arns.FromArn == nil {
					arns.FromArn = arns.SourceArn
				}
				if arns.ReturnPathArn == nil {
					arns.ReturnPathArn = arns.SourceArn
				}
			}
		}
		client = sesrelay.New(&cfg.SetName, cfg.AllowFromRegExp, cfg.DenyToRegExp, cfg.AllowToRegExp, cfg.AllowToDomainsSlice, uint(cfg.MaxMessageBytes), arns)
	default:
		return nil, errors.New("Invalid relay API: " + cfg.RelayAPI)
	}
	return client, nil
}
