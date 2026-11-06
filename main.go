package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"strconv"
	"log"

	"github.com/KamorionLabs/aws-smtp-relay/internal/auth"
	"github.com/KamorionLabs/aws-smtp-relay/internal/metrics"
	"github.com/KamorionLabs/aws-smtp-relay/internal/relay"
	pinpointrelay "github.com/KamorionLabs/aws-smtp-relay/internal/relay/pinpoint"
	sesrelay "github.com/KamorionLabs/aws-smtp-relay/internal/relay/ses"
	"github.com/mhale/smtpd"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	addr          = flag.String("a", LookupEnvOrString("LISTEN_ADDRESS", ":1025"), "TCP listen address")
	name          = flag.String("n", LookupEnvOrString("SMTP_SERVICE_NAME", "AWS SMTP Relay"), "SMTP service name")
	host          = flag.String("h", LookupEnvOrString("HOSTNAME", ""), "Server hostname")
	certFile      = flag.String("c", LookupEnvOrString("CERT_FILE", ""), "TLS cert file")
	keyFile       = flag.String("k", LookupEnvOrString("KEY_FILE", ""), "TLS key file")
	startTLS      = flag.Bool("s", LookupEnvOrBool("REQUIRE_STARTTLS", false), "Require TLS via STARTTLS extension")
	onlyTLS       = flag.Bool("t", LookupEnvOrBool("REQUIRE_TLS", false), "Listen for incoming TLS connections only")
	relayAPI      = flag.String("r", LookupEnvOrString("RELAY_API", "ses"), "Relay API to use (ses|pinpoint)")
	setName       = flag.String("e", LookupEnvOrString("SES_CONFIGURATION_SET_NAME", ""), "Amazon SES Configuration Set Name")
	ips           = flag.String("i", LookupEnvOrString("ALLOWED_IPS", ""), "Allowed client IPs (comma-separated)")
	user          = flag.String("u", LookupEnvOrString("AUTH_USERNAME", ""), "Authentication username")
	allowFrom     = flag.String("l", LookupEnvOrString("ALLOWED_SENDERS_REGEX", ""), "Allowed sender emails regular expression")
	denyTo        = flag.String("d", LookupEnvOrString("DENIED_RECIPIENTS_REGEX", ""), "Denied recipient emails regular expression")
	sourceArn     = flag.String("o", LookupEnvOrString("SES_SOURCE_ARN", ""), "Amazon SES SourceArn")
	fromArn       = flag.String("f", LookupEnvOrString("SES_FROM_ARN", ""), "Amazon SES FromArn")
	returnPathArn = flag.String("p", LookupEnvOrString("SES_RETURN_PATH_ARN", ""), "Amazon SES ReturnPathArn")
	metricsAddr   = flag.String("m", LookupEnvOrString("METRICS_ADDRESS", ""), "HTTP listen address for /metrics and /healthz (off when empty)")
)

var ipMap map[string]bool
var denyToRegExp *regexp.Regexp
var bcryptHash []byte
var password []byte
var relayClient relay.Client

// handler sends the mail and gives the client a reply code that tells it
// whether to try again.
func handler(origin net.Addr, from string, to []string, data []byte) error {
	return relay.Reply(relayClient.Send(origin, from, to, data))
}

// startMetrics counts the messages and failed logins of srv, and serves the
// counts on addr in the background. A failure of that server does not stop
// the relay.
func startMetrics(srv *smtpd.Server, addr string, reg prometheus.Registerer, g prometheus.Gatherer) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return errors.New("Metrics address: " + err.Error())
	}
	m := metrics.New(reg)
	srv.Handler = m.Handler(srv.Handler)
	srv.AuthHandler = m.AuthHandler(srv.AuthHandler)
	go func() {
		log.Printf("Metrics server stopped: %v\r\n", metrics.Serve(ln, g))
	}()
	log.Printf("Serving metrics on %v\r\n", ln.Addr())
	return nil
}

// toStringPtr returns nil for empty strings, otherwise returns a pointer to the string
func toStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func server() (srv *smtpd.Server, err error) {
	authMechs := make(map[string]bool)
	if *user != "" && len(bcryptHash) > 0 && len(password) == 0 {
		authMechs["CRAM-MD5"] = false
	}

	// force-enable the login mechanism: only needed if there is no external routing and TLS is not configured
	if LookupEnvOrString("ENABLE_LOGIN", "") == "true" {
		authMechs["LOGIN"] = true
	}
	
	version := LookupEnvOrString("GIT_REV", "")
	if version == "" {
		log.Printf("Listening on %v\r\n", *addr)
	}else{
		log.Printf("(Revision: %s) Listening on %v\r\n", version, *addr)
	}
	srv = &smtpd.Server{
		Addr:         *addr,
		Handler:      handler,
		HandlerRcpt:  relay.RcptHandler(denyToRegExp),
		Appname:      *name,
		Hostname:     *host,
		TLSRequired:  *startTLS,
		TLSListener:  *onlyTLS,
		AuthRequired: ipMap != nil || *user != "",
		AuthHandler:  auth.New(ipMap, *user, bcryptHash, password).Handler,
		AuthMechs:    authMechs,
	}
	if *certFile != "" && *keyFile != "" {
		keyPass := os.Getenv("TLS_KEY_PASS")
		if keyPass != "" {
			err = srv.ConfigureTLSWithPassphrase(*certFile, *keyFile, keyPass)
		} else {
			err = srv.ConfigureTLS(*certFile, *keyFile)
		}
	}
	return
}

func configure() error {
	var allowFromRegExp *regexp.Regexp
	var err error
	if *allowFrom != "" {
		allowFromRegExp, err = regexp.Compile(*allowFrom)
		if err != nil {
			return errors.New("Allowed sender emails: " + err.Error())
		}
	}
	if *denyTo != "" {
		denyToRegExp, err = regexp.Compile(*denyTo)
		if err != nil {
			return errors.New("Denied recipient emails: " + err.Error())
		}
	}
	var arns *relay.ARNs
	// Configure ARNs for cross-account authorization if any ARN is provided
	if *sourceArn != "" || *fromArn != "" || *returnPathArn != "" {
		arns = &relay.ARNs{
			SourceArn:     toStringPtr(*sourceArn),
			FromArn:       toStringPtr(*fromArn),
			ReturnPathArn: toStringPtr(*returnPathArn),
		}
		// If SourceArn is provided, use it as default for FromArn and ReturnPathArn
		if *sourceArn != "" {
			if arns.FromArn == nil {
				arns.FromArn = arns.SourceArn
			}
			if arns.ReturnPathArn == nil {
				arns.ReturnPathArn = arns.SourceArn
			}
		}
	}
	switch *relayAPI {
	case "pinpoint":
		relayClient = pinpointrelay.New(setName, allowFromRegExp, denyToRegExp)
	case "ses":
		relayClient = sesrelay.New(setName, allowFromRegExp, denyToRegExp, arns)
	default:
		return errors.New("Invalid relay API: " + *relayAPI)
	}
	if *ips != "" {
		ipMap = make(map[string]bool)
		for _, ip := range strings.Split(*ips, ",") {
			ipMap[ip] = true
		}
	}
	bcryptHash = []byte(os.Getenv("BCRYPT_HASH"))
	password = []byte(os.Getenv("PASSWORD"))
	return nil
}

// returns the value of an environment variable or the default value
func LookupEnvOrString(key string, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func LookupEnvOrInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok {
		v, err := strconv.Atoi(val)
		if err != nil {
			log.Fatalf("LookupEnvOrInt[%s]: %v", key, err)
		}
		return v
	}
	return defaultVal
}

func LookupEnvOrBool(key string, defaultVal bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		val = strings.ToLower(val)
		if val == "true" {
			return true
		}else if val == "false" {
			return false
		}
		log.Fatalf("LookupEnvOrBool[%s]: invalid value %v", key, val)
	}
	return defaultVal
}

func main() {
	flag.Parse()
	var srv *smtpd.Server
	err := configure()
	if err == nil {
		srv, err = server()
		if err == nil && *metricsAddr != "" {
			err = startMetrics(srv, *metricsAddr, prometheus.DefaultRegisterer, prometheus.DefaultGatherer)
		}
		if err == nil {
			err = srv.ListenAndServe()
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
