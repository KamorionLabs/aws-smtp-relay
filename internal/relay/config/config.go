package config

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/KamorionLabs/aws-smtp-relay/internal"
	"github.com/spf13/pflag"
)

type Config struct {
	Addr            string
	Name            string
	Host            string
	ReadTimeout     int64
	WriteTimeout    int64
	MaxMessageBytes uint32
	CertFile        string
	KeyFile         string
	StartTLSStr     string
	OnlyTLSStr      string
	RelayAPI        string
	SetName         string
	Ips             string
	User            string
	Debug           string
	IpMap           map[string]bool
	BcryptHash      []byte
	Password        []byte

	AllowFrom           string
	AllowFromRegExp     *regexp.Regexp
	DenyTo              string
	DenyToRegExp        *regexp.Regexp
	AllowTo             string
	AllowToRegExp       *regexp.Regexp
	AllowToDomains      string
	AllowToDomainsSlice []string

	SourceArn     string
	FromArn       string
	ReturnPathArn string
}

func (c Config) StartTLS() bool {
	return internal.String2bool(c.StartTLSStr)
}

func (c Config) OnlyTLS() bool {
	return internal.String2bool(c.OnlyTLSStr)
}

func initCliArgs() *Config {
	cfg := Config{}
	pflag.StringVarP(&cfg.Addr, "addr", "a", ":1025", "TCP listen address")
	pflag.StringVarP(&cfg.Name, "name", "n", "AWS SMTP Relay", "SMTP service name")
	pflag.StringVarP(&cfg.Host, "host", "h", "", "Server hostname")
	pflag.StringVarP(&cfg.CertFile, "certFile", "c", "", "TLS cert file")
	pflag.StringVarP(&cfg.KeyFile, "keyFile", "k", "", "TLS key file")
	pflag.StringVarP(&cfg.StartTLSStr, "startTLS", "s", "false", "Require TLS via STARTTLS extension")
	pflag.StringVarP(&cfg.OnlyTLSStr, "onlyTLS", "t", "false", "Listen for incoming TLS connections only")
	pflag.StringVarP(&cfg.RelayAPI, "relayAPI", "r", "ses", "Relay API to use (ses|pinpoint)")
	pflag.StringVarP(&cfg.SetName, "setName", "e", "", "Amazon SES Configuration Set Name")
	pflag.StringVarP(&cfg.Ips, "ips", "i", "", "Allowed client IPs (comma-separated)")
	pflag.StringVarP(&cfg.User, "user", "u", "", "Authentication username")
	pflag.StringVarP(&cfg.AllowFrom, "allowFrom", "l", "", "Allowed sender emails regular expression")
	pflag.StringVarP(&cfg.DenyTo, "denyTo", "d", "", "Denied recipient emails regular expression")
	pflag.StringVarP(&cfg.AllowTo, "allowTo", "w", "", "Allowed recipient emails regular expression")
	pflag.StringVarP(&cfg.AllowToDomains, "allowToDomains", "m", "", "Allowed recipient domains (comma-separated)")
	pflag.StringVarP(&cfg.SourceArn, "sourceArn", "o", "", "Amazon SES SourceArn")
	pflag.StringVarP(&cfg.FromArn, "fromArn", "f", "", "Amazon SES FromArn")
	pflag.StringVarP(&cfg.ReturnPathArn, "returnPathArn", "p", "", "Amazon SES ReturnPathArn")
	pflag.Int64VarP(&cfg.ReadTimeout, "readTimeout", "", int64(1*time.Minute), "Read timeout in seconds")
	pflag.Int64VarP(&cfg.WriteTimeout, "writeTimeout", "", int64(1*time.Minute), "Write timeout in seconds")
	pflag.Uint32VarP(&cfg.MaxMessageBytes, "maxMessageBytes", "", 10*1024*1024, "Max Session Size")
	pflag.StringVarP(&cfg.Debug, "debug", "", "", "Debug File")
	return &cfg
}

var FlagCliArgs = initCliArgs()

func merge(dominator, defaults Config) Config {
	if dominator.Addr == "" {
		dominator.Addr = defaults.Addr
	}
	if dominator.Name == "" {
		dominator.Name = defaults.Name
	}
	if dominator.Host == "" {
		dominator.Host = defaults.Host
	}
	if dominator.CertFile == "" {
		dominator.CertFile = defaults.CertFile
	}
	if dominator.KeyFile == "" {
		dominator.KeyFile = defaults.KeyFile
	}
	if dominator.RelayAPI == "" {
		dominator.RelayAPI = defaults.RelayAPI
	}
	if dominator.SetName == "" {
		dominator.SetName = defaults.SetName
	}
	if dominator.Ips == "" {
		dominator.Ips = defaults.Ips
	}
	if dominator.User == "" {
		dominator.User = defaults.User
	}
	if dominator.AllowFrom == "" {
		dominator.AllowFrom = defaults.AllowFrom
	}
	if dominator.DenyTo == "" {
		dominator.DenyTo = defaults.DenyTo
	}
	if dominator.AllowTo == "" {
		dominator.AllowTo = defaults.AllowTo
	}
	if dominator.AllowToDomains == "" {
		dominator.AllowToDomains = defaults.AllowToDomains
	}
	if dominator.ReadTimeout == 0 {
		dominator.ReadTimeout = defaults.ReadTimeout
	}
	if dominator.WriteTimeout == 0 {
		dominator.WriteTimeout = defaults.WriteTimeout
	}
	if dominator.MaxMessageBytes == 0 {
		dominator.MaxMessageBytes = defaults.MaxMessageBytes
	}
	if dominator.Debug == "" {
		dominator.Debug = defaults.Debug
	}
	if len(dominator.Password) == 0 {
		dominator.Password = defaults.Password
	}
	if len(dominator.BcryptHash) == 0 {
		dominator.BcryptHash = defaults.BcryptHash
	}
	return dominator
}

func Configure(clis ...Config) (*Config, error) {
	incli := *FlagCliArgs
	if len(clis) != 0 {
		incli = clis[0]
	}
	// own copy
	cli := merge(incli, *FlagCliArgs)

	var err error
	if cli.AllowFrom != "" {
		cli.AllowFromRegExp, err = regexp.Compile(cli.AllowFrom)
		if err != nil {
			return nil, errors.New("Allowed sender emails: " + err.Error())
		}
	}
	if cli.DenyTo != "" {
		cli.DenyToRegExp, err = regexp.Compile(cli.DenyTo)
		if err != nil {
			return nil, errors.New("Denied recipient emails: " + err.Error())
		}
	}
	if cli.AllowTo != "" {
		cli.AllowToRegExp, err = regexp.Compile(cli.AllowTo)
		if err != nil {
			return nil, errors.New("Allowed recipient emails: " + err.Error())
		}
	}
	if cli.AllowToDomains != "" {
		cli.AllowToDomainsSlice = []string{}
		for _, domain := range strings.Split(cli.AllowToDomains, ",") {
			domain = strings.TrimSpace(domain)
			if domain != "" {
				cli.AllowToDomainsSlice = append(cli.AllowToDomainsSlice, domain)
			}
		}
	}

	cli.IpMap = make(map[string]bool)
	if cli.Ips != "" {
		for _, ip := range strings.Split(cli.Ips, ",") {
			cli.IpMap[ip] = true
		}
	}

	// Load credentials from environment variables
	bh, ok := os.LookupEnv("BCRYPT_HASH")
	if ok {
		cli.BcryptHash = []byte(bh)
	}
	pw, ok := os.LookupEnv("PASSWORD")
	if ok {
		cli.Password = []byte(pw)
	}

	// Load filtering configuration from environment variables
	if allowTo, ok := os.LookupEnv("ALLOW_TO"); ok && allowTo != "" {
		cli.AllowTo = allowTo
		cli.AllowToRegExp, err = regexp.Compile(cli.AllowTo)
		if err != nil {
			return nil, errors.New("ALLOW_TO environment variable: " + err.Error())
		}
	}
	if allowToDomains, ok := os.LookupEnv("ALLOW_TO_DOMAINS"); ok && allowToDomains != "" {
		cli.AllowToDomains = allowToDomains
		cli.AllowToDomainsSlice = []string{}
		for _, domain := range strings.Split(cli.AllowToDomains, ",") {
			domain = strings.TrimSpace(domain)
			if domain != "" {
				cli.AllowToDomainsSlice = append(cli.AllowToDomainsSlice, domain)
			}
		}
	}

	return &cli, nil
}
