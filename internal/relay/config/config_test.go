package config

import (
	"os"
	"testing"
)

func TestConfigure(t *testing.T) {
	cfg, err := Configure()
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	if cfg.User != "" {
		t.Errorf("Unexpected username: %s", cfg.User)
	}
	if string(cfg.BcryptHash) != "" {
		t.Errorf("Unexpected bhash: %s", string(cfg.BcryptHash))
	}
	if cfg.Ips != "" {
		t.Errorf("Unexpected IPs string: %s", cfg.Ips)
	}
	if len(cfg.IpMap) != 0 {
		t.Errorf("Unexpected IP map size: %d", len(cfg.IpMap))
	}
	if cfg.RelayAPI != "ses" {
		t.Errorf("Unexpected relay API: %s", cfg.RelayAPI)
	}
	if string(cfg.BcryptHash) != "" {
		t.Errorf("Unexpected bhash: %s", string(cfg.BcryptHash))
	}
}

func TestConfigureWithBcryptHash(t *testing.T) {
	sampleHash := "$2y$10$85/eICRuwBwutrou64G5HeoF3Ek/qf1YKPLba7ckiMxUTAeLIeyaC"
	os.Setenv("BCRYPT_HASH", sampleHash)
	cfg, err := Configure()
	os.Unsetenv("BCRYPT_HASH")
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	if string(cfg.BcryptHash) != sampleHash {
		t.Errorf("Unexpected bhash: %s", string(cfg.BcryptHash))
	}
}

func TestConfigureWithPassword(t *testing.T) {
	os.Setenv("PASSWORD", "password")
	cfg, err := Configure()
	os.Unsetenv("PASSWORD")
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	if string(cfg.Password) != "password" {
		t.Errorf("Unexpected password: %s", string(cfg.Password))
	}
}

func TestConfigureWithAllowTo(t *testing.T) {
	os.Setenv("ALLOW_TO", "@example\\.org$")
	cfg, err := Configure()
	os.Unsetenv("ALLOW_TO")
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	if cfg.AllowTo != "@example\\.org$" {
		t.Errorf("Unexpected allowTo: %s", cfg.AllowTo)
	}
	if cfg.AllowToRegExp == nil {
		t.Error("AllowToRegExp should not be nil")
	}
}

func TestConfigureWithAllowToDomains(t *testing.T) {
	os.Setenv("ALLOW_TO_DOMAINS", "example.org,example.com")
	cfg, err := Configure()
	os.Unsetenv("ALLOW_TO_DOMAINS")
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	if cfg.AllowToDomains != "example.org,example.com" {
		t.Errorf("Unexpected allowToDomains: %s", cfg.AllowToDomains)
	}
	if len(cfg.AllowToDomainsSlice) != 2 {
		t.Errorf("Expected 2 domains, got %d", len(cfg.AllowToDomainsSlice))
	}
	if cfg.AllowToDomainsSlice[0] != "example.org" || cfg.AllowToDomainsSlice[1] != "example.com" {
		t.Errorf("Unexpected domain slice: %v", cfg.AllowToDomainsSlice)
	}
}

func TestConfigureWithInvalidAllowTo(t *testing.T) {
	os.Setenv("ALLOW_TO", "(")
	_, err := Configure()
	os.Unsetenv("ALLOW_TO")
	if err == nil {
		t.Error("Expected error for invalid ALLOW_TO regex")
	}
}

func TestConfigureWithIPs(t *testing.T) {
	cfg, err := Configure(Config{
		Ips: "127.0.0.1,2001:4860:0:2001::68",
	})
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	if len(cfg.IpMap) != 2 {
		t.Errorf("Unexpected IP map size: %d", len(cfg.IpMap))
	}
}

func TestConfigureWithAllowFrom(t *testing.T) {
	_, err := Configure(Config{
		AllowFrom: "^admin@example\\.org$",
	})
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
}

func TestConfigureWithInvalidAllowFrom(t *testing.T) {
	_, err := Configure(Config{
		AllowFrom: "(",
	})
	if err == nil {
		t.Error("Unexpected nil error")
	}
}

func TestConfigureWithDenyTo(t *testing.T) {
	_, err := Configure(Config{
		DenyTo: "^bob@example\\.org$",
	})
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
}

func TestConfigureWithInvalidDenyTo(t *testing.T) {
	_, err := Configure(Config{
		DenyTo: "(",
	})
	if err == nil {
		t.Error("Unexpected nil error")
	}
}

func TestMerge(t *testing.T) {
	defaults := Config{
		Addr:     ":1025",
		Name:     "Default Name",
		User:     "defaultuser",
		AllowTo:  "default",
		RelayAPI: "ses",
	}

	dominator := Config{
		Addr: ":2525",
		User: "customuser",
	}

	result := merge(dominator, defaults)

	if result.Addr != ":2525" {
		t.Errorf("Expected Addr :2525, got %s", result.Addr)
	}
	if result.User != "customuser" {
		t.Errorf("Expected User customuser, got %s", result.User)
	}
	if result.Name != "Default Name" {
		t.Errorf("Expected Name from defaults, got %s", result.Name)
	}
	if result.AllowTo != "default" {
		t.Errorf("Expected AllowTo from defaults, got %s", result.AllowTo)
	}
	if result.RelayAPI != "ses" {
		t.Errorf("Expected RelayAPI from defaults, got %s", result.RelayAPI)
	}
}

func TestConfigureWithAllowToDomainsSpaces(t *testing.T) {
	cfg, err := Configure(Config{
		AllowToDomains: " example.org , example.com , ",
	})
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	if len(cfg.AllowToDomainsSlice) != 2 {
		t.Errorf("Expected 2 domains after trimming, got %d", len(cfg.AllowToDomainsSlice))
	}
}
