package factory

import (
	"reflect"
	"testing"

	"github.com/KamorionLabs/aws-smtp-relay/internal/relay/config"
)

func TestConfigureWithPinpointRelay(t *testing.T) {
	cfg, err := config.Configure(config.Config{
		RelayAPI: "pinpoint",
	})
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	typ := reflect.TypeOf(client).String()
	if typ != "pinpoint.Client" {
		t.Errorf("Unexpected type: %s, expected pinpoint.Client", typ)
	}
}

func TestConfigureWithSesRelay(t *testing.T) {
	cfg, err := config.Configure(config.Config{
		RelayAPI: "ses",
	})
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	typ := reflect.TypeOf(client).String()

	if typ != "ses.Client" {
		t.Errorf("Unexpected type: %s, expected ses.Client", typ)
	}
}

func TestConfigureWithInvalidRelay(t *testing.T) {
	cfg, err := config.Configure(config.Config{
		RelayAPI: "invalid",
	})
	if err != nil {
		t.Errorf("Unexpected error during config: %s", err)
	}
	_, err = NewClient(cfg)
	if err == nil {
		t.Error("Expected error for invalid relay API")
	}
}

func TestNewClientWithARNs(t *testing.T) {
	cfg, err := config.Configure(config.Config{
		RelayAPI:      "ses",
		SourceArn:     "arn:aws:ses:us-east-1:123456789012:identity/example.com",
		FromArn:       "arn:aws:ses:us-east-1:123456789012:identity/from.example.com",
		ReturnPathArn: "arn:aws:ses:us-east-1:123456789012:identity/return.example.com",
	})
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	typ := reflect.TypeOf(client).String()
	if typ != "ses.Client" {
		t.Errorf("Unexpected type: %s", typ)
	}
}

func TestNewClientWithSourceArnOnly(t *testing.T) {
	cfg, err := config.Configure(config.Config{
		RelayAPI:  "ses",
		SourceArn: "arn:aws:ses:us-east-1:123456789012:identity/example.com",
	})
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	typ := reflect.TypeOf(client).String()
	if typ != "ses.Client" {
		t.Errorf("Unexpected type: %s", typ)
	}
}

func TestToStringPtr(t *testing.T) {
	// Test with non-empty string
	str := "test"
	ptr := toStringPtr(str)
	if ptr == nil {
		t.Error("Expected non-nil pointer for non-empty string")
	}
	if *ptr != str {
		t.Errorf("Expected %s, got %s", str, *ptr)
	}

	// Test with empty string
	emptyPtr := toStringPtr("")
	if emptyPtr != nil {
		t.Error("Expected nil pointer for empty string")
	}
}
