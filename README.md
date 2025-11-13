# AWS SMTP Relay

> SMTP server to relay emails via Amazon SES or Amazon Pinpoint using IAM roles.

## ⚠️ Fork Notice

This repository is a maintained fork of [blueimp/aws-smtp-relay](https://github.com/blueimp/aws-smtp-relay), which appears to be inactive. This fork includes:

- **Active maintenance** and support for pending pull requests
- **Cross-account SES authorization** support via ARN parameters
- **Multi-architecture Docker images** (amd64, arm64)
- **Dual registry publishing** to Docker Hub and GitHub Container Registry

Pull requests and contributions are welcome!

**Contents**

- [Background](#background)
- [Docker](#docker)
- [Installation](#installation)
- [Usage](#usage)
  - [Options](#options)
  - [Authentication](#authentication)
    - [User](#user)
    - [IP](#ip)
  - [TLS](#tls)
  - [Filtering](#filtering)
    - [Senders](#senders)
    - [Recipients](#recipients)
  - [Region](#region)
  - [Credentials](#credentials)
  - [Logging](#logging)
- [Development](#development)
  - [Build](#build)
  - [Lint](#lint)
  - [Test](#test)
  - [Install](#install)
  - [Uninstall](#uninstall)
  - [Clean](#clean)
- [Dependencies](#dependencies)
- [License](#license)

## Background

[Amazon SES](https://docs.aws.amazon.com/ses/latest/DeveloperGuide/Welcome.html)
and
[Amazon Pinpoint](https://docs.aws.amazon.com/pinpoint/latest/developerguide/welcome.html)
both provide an API and an SMTP interface to send emails:

- [SES Email API](https://docs.aws.amazon.com/ses/latest/DeveloperGuide/send-email-api.html)
- [SES SMTP interface](https://docs.aws.amazon.com/ses/latest/DeveloperGuide/send-email-smtp.html)
- [Pinpoint EmailAPI](https://docs.aws.amazon.com/pinpoint/latest/developerguide/send-messages-email-sdk.html)
- [Pinpoint SMTP interface](https://docs.aws.amazon.com/pinpoint/latest/developerguide/send-messages-email-smtp.html)

The SMTP interface is useful for applications that must use SMTP to send emails,
but it requires providing a set of SMTP credentials:

- [SES SMTP Credentials](https://docs.aws.amazon.com/ses/latest/DeveloperGuide/smtp-credentials.html)
- [Pinpoint SMTP Credentials](https://docs.aws.amazon.com/pinpoint/latest/userguide/channels-email-send-smtp.html#channels-email-send-smtp-credentials)

For security reasons, using
[IAM roles](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles.html) is
preferable, but only possible with the Email API and not the SMTP interface.

This is where this project comes into play, as it provides an SMTP interface
that relays emails via SES or Pinpoint API using IAM roles.

## Docker

This repository provides [Dockerfile](Dockerfile) and [Dockerfile.alpine](Dockerfile.alpine) to build and run the project in a container environment.

Prebuilt multi-architecture Docker images (amd64, arm64, armv7) are available on both registries:

- **Docker Hub**: [kamorion/aws-smtp-relay](https://hub.docker.com/r/kamorion/aws-smtp-relay/)
- **GitHub Container Registry**: `ghcr.io/kamorionlabs/aws-smtp-relay`

### Available Image Tags

**Production (Distroless - Recommended)**

Minimal, secure images based on Google's Distroless base (no shell, no package manager):

```sh
# Stable releases
docker pull kamorion/aws-smtp-relay:latest          # Latest stable release
docker pull kamorion/aws-smtp-relay:v1.2.3          # Specific version
docker pull kamorion/aws-smtp-relay:v1.2            # Latest patch of v1.2
docker pull kamorion/aws-smtp-relay:v1              # Latest minor of v1

# Development (from main branch)
docker pull kamorion/aws-smtp-relay:edge            # Latest commit from main branch
```

**Debug (Alpine - With Shell)**

Images based on Alpine Linux, includes shell and basic debugging tools:

```sh
# Stable releases
docker pull kamorion/aws-smtp-relay:latest-alpine   # Latest stable release
docker pull kamorion/aws-smtp-relay:v1.2.3-alpine   # Specific version

# Development (from main branch)
docker pull kamorion/aws-smtp-relay:edge-alpine     # Latest commit from main branch
```

**Notes:**
- All tags are available on both Docker Hub (`kamorion/`) and GHCR (`ghcr.io/kamorionlabs/`)
- `latest` tag = last stable release (tagged with `v*`)
- `edge` tag = latest commit on main branch (bleeding edge, for testing)
- Use **distroless** images for production (smaller, more secure)
- Use **alpine** images for debugging (includes shell access)

### Quick Start

```sh
# Production (recommended)
docker run -d \
  -p 1025:1025 \
  -e AWS_REGION=eu-west-1 \
  kamorion/aws-smtp-relay:latest

# Debug with shell access
docker run -d \
  -p 1025:1025 \
  -e AWS_REGION=eu-west-1 \
  kamorion/aws-smtp-relay:latest-alpine

# Test bleeding edge
docker run -d \
  -p 1025:1025 \
  -e AWS_REGION=eu-west-1 \
  kamorion/aws-smtp-relay:edge
```

## Installation

### Pre-compiled Binaries

Download pre-compiled binaries for your platform from [GitHub Releases](https://github.com/KamorionLabs/aws-smtp-relay/releases).

Available platforms:
- Linux (amd64, arm64, armv6, armv7)
- Windows (amd64, arm64)
- macOS (amd64, arm64)

### From Source

The `aws-smtp-relay` binary can be installed from source via [go install](https://golang.org/cmd/go/):

```sh
go install github.com/KamorionLabs/aws-smtp-relay@latest
```

### Docker

See [Docker section](#docker) above for container images.

## Usage

By default, `aws-smtp-relay` listens on port `1025` on all interfaces as open
relay (without authentication) when started without arguments:

```sh
aws-smtp-relay
```

### Options

Available options can be listed the following way:

```sh
aws-smtp-relay --help
```

```
Usage of aws-smtp-relay:
  -a string
        TCP listen address (default ":1025")
  -c string
        TLS cert file
  -d string
        Denied recipient emails regular expression
  -e string
        Amazon SES Configuration Set Name
  -f string
        Amazon SES FromArn
  -h string
        Server hostname
  -i string
        Allowed client IPs (comma-separated)
  -k string
        TLS key file
  -l string
        Allowed sender emails regular expression
  -m string
        Allowed recipient domains (comma-separated)
  -n string
        SMTP service name (default "AWS SMTP Relay")
  -o string
        Amazon SES SourceArn
  -p string
        Amazon SES ReturnPathArn
  -r string
        Relay API to use (ses|pinpoint) (default "ses")
  -s    Require TLS via STARTTLS extension
  -t    Listen for incoming TLS connections only
  -u string
        Authentication username
  -w string
        Allowed recipient emails regular expression
```

### Authentication

#### User

The supported user-based SMTP authentication mechanisms and their required
configuration settings (see also
[RFC 4954](https://tools.ietf.org/html/rfc4954#section-9)):

| Mechanism  | [TLS](#tls) | User | Hash | Pass |
| ---------- | ----------- | ---- | ---- | ---- |
| `LOGIN`    | Yes         | Yes  | Yes  | No   |
| `PLAIN`    | Yes         | Yes  | Yes  | No   |
| `CRAM-MD5` | No          | Yes  | No   | Yes  |

Authentication can be enabled for `LOGIN` and `PLAIN` mechanisms by configuring
[TLS](#tls) and a username and providing the
[bcrypt](https://en.wikipedia.org/wiki/Bcrypt) encrypted password as
`BCRYPT_HASH` environment variable:

```sh
export BCRYPT_HASH=$(htpasswd -bnBC 10 '' password | tr -d ':\n')
export TLS_KEY_PASS="$PASSPHRASE"

aws-smtp-relay -c tls/default.crt -k tls/default.key -u username
```

If the password is provided as plain text `PASSWORD` environment variable, it
will also enable the `CRAM-MD5` authentication mechanism:

```sh
export PASSWORD=password
export TLS_KEY_PASS="$PASSPHRASE"

aws-smtp-relay -c tls/default.crt -k tls/default.key -u username
```

Without [TLS](#tls) configuration, only `CRAM-MD5` will be enabled:

```sh
export PASSWORD=password

aws-smtp-relay -u username
```

**Please note**:

> It is not recommended to provide the password as plain text environment
> variable, nor to configure the SMTP server without [TLS](#tls) support.

#### IP

To limit the allowed IP addresses, supply a comma-separated list via `-i ips`
option:

```sh
aws-smtp-relay -i 127.0.0.1,::1
```

**Please note**:

> To authorize their IP, clients must use a supported SMTP authentication
> mechanism, e.g. `LOGIN` or `PLAIN` via [TLS](#tls) or `CRAM-MD5` on
> unencrypted connections.
> This is required even if no user authentication is configured on the server,
> although in this case the credentials can be chosen freely by the client.

### TLS

Configure [TLS](https://en.wikipedia.org/wiki/Transport_Layer_Security) with the
following steps:

Edit the [openssl config file](tls/openssl.conf) and change `localhost` to your
server hostname.

Generate a self-signed certificate with a passphrase encrypted key:

```sh
openssl req -new -x509 -config tls/openssl.conf -days 24855 \
  -out tls/default.crt \
  -keyout /dev/stdout |
  openssl rsa -aes256 -out tls/default.key
```

**Please note**:

> Encrypted key files are only supported if they contain a `DEK-Info` header,
> stating the encryption method used.
> The `openssl req` command does not create this header if encryption is
> enabled, which is why we pipe the unencrypted key output to the `openssl rsa`
> command, which outputs an encrypted key file with the required `DEK-Info`
> header.

Provide the key file passphrase as `TLS_KEY_PASS` environment variable and the
cert and key file as command-line arguments:

```sh
TLS_KEY_PASS="$PASSPHRASE" aws-smtp-relay -c tls/default.crt -k tls/default.key
```

**Please note**:

> It is recommended to require TLS via `STARTTLS` extension (`-s` option flag)
> or to configure the server to listen for incoming TLS connections only (`-t`
> option flag).

### Filtering

All filtering options can be configured either via command-line flags or environment variables.

#### Senders

To limit the allowed sender email addresses, provide an allow list as
[regular expression](https://golang.org/pkg/regexp/syntax/) via `-l regexp`
option or `ALLOW_FROM` environment variable:

```sh
# Via command-line flag
aws-smtp-relay -l '@example\.org$'

# Via environment variable
export ALLOW_FROM='@example\.org$'
aws-smtp-relay
```

By default, all sender email addresses are allowed.

#### Recipients

**Blacklist Filtering (Deny List)**

To deny certain recipient email addresses, provide a deny list as
[regular expression](https://golang.org/pkg/regexp/syntax/) via `-d regexp`
option or `DENY_TO` environment variable:

```sh
# Via command-line flag
aws-smtp-relay -d 'admin@example\.org$'

# Via environment variable
export DENY_TO='admin@example\.org$'
aws-smtp-relay
```

**Whitelist Filtering (Allow List)**

To allow only specific recipient email addresses, provide an allow list as
[regular expression](https://golang.org/pkg/regexp/syntax/) via `-w regexp`
option or `ALLOW_TO` environment variable:

```sh
# Via command-line flag
aws-smtp-relay -w '@example\.org$'

# Via environment variable
export ALLOW_TO='@example\.org$'
aws-smtp-relay
```

**Domain Whitelist Filtering**

To allow only specific recipient domains, provide a comma-separated list via `-m domains`
option or `ALLOW_TO_DOMAINS` environment variable:

```sh
# Via command-line flag
aws-smtp-relay -m example.org,example.com

# Via environment variable
export ALLOW_TO_DOMAINS='example.org,example.com'
aws-smtp-relay
```

**Filtering Precedence**

When multiple recipient filters are configured, they are applied in the following order:

1. **Sender validation**: If `-l` (allowFrom) is set, sender must match the pattern
2. **Blacklist**: If `-d` (denyTo) is set, recipients matching this pattern are denied
3. **Whitelist**: If `-w` (allowTo) is set, recipients must match this pattern
4. **Domain whitelist**: If `-m` (allowToDomains) is set, recipients must have domains in this list

All filters must pass for a recipient to be allowed. For example:

```sh
# Allow only recipients from example.org or example.com domains, except admin@example.org
aws-smtp-relay -m example.org,example.com -d '^admin@example\.org$'
```

By default, all recipient email addresses are allowed (no filters applied).

### Cross-Account Authorization

For cross-account SES authorization, you can specify Amazon Resource Names (ARNs):

```sh
aws-smtp-relay -o arn:aws:ses:region:account-id:identity/example.com
```

Available ARN options:

- `-o` : Amazon SES SourceArn (also used as default for FromArn and ReturnPathArn if not specified)
- `-f` : Amazon SES FromArn
- `-p` : Amazon SES ReturnPathArn

If only SourceArn (`-o`) is provided, it will automatically be used for FromArn and ReturnPathArn unless explicitly overridden.

### SESv2 API ARN Mapping

This relay uses the **Amazon SES v2 API** which maps ARN parameters as follows:

- **FromArn** or **SourceArn** → [`FromEmailAddressIdentityArn`](https://docs.aws.amazon.com/ses/latest/APIReference-V2/API_SendEmail.html#API_SendEmail_RequestSyntax)
  Used for sending authorization when the identity owner has granted you permission to send on their behalf.

- **ReturnPathArn** → [`FeedbackForwardingEmailAddressIdentityArn`](https://docs.aws.amazon.com/ses/latest/APIReference-V2/API_SendEmail.html#API_SendEmail_RequestSyntax)
  Used for bounce and complaint notifications. The ARN of the identity associated with the feedback forwarding address.

For more information about SESv2 sending authorization, see the [Amazon SES Developer Guide](https://docs.aws.amazon.com/ses/latest/DeveloperGuide/sending-authorization.html).

See [AWS SES Cross-Account Sending](https://docs.aws.amazon.com/ses/latest/dg/sending-authorization.html) for more details.

### Region

The `AWS_REGION` must be set to configure the AWS SDK, e.g. by executing the
following command before starting `aws-smtp-relay`:

```sh
export AWS_REGION=eu-west-1
```

### Credentials

On [EC2](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/concepts.html) or
[ECS](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/Welcome.html),
security credentials for the IAM role are automatically retrieved:

- [IAM Roles for Amazon EC2](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html)
- [IAM Roles for Tasks](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-iam-roles.html)

### Logging

Requests are logged in `JSON` format to `stdout` with the `Error` property set
to `null`:

```json
{
  "Time": "2018-04-18T15:08:42.4388893Z",
  "IP": "172.17.0.1",
  "From": "alice@example.org",
  "To": ["bob@example.org"],
  "Error": null
}
```

Errors are logged in the same format to `stderr`, with the `Error` property set
to a `string` value:

```json
{
  "Time": "2018-04-18T15:08:42.4388893Z",
  "IP": "172.17.0.1",
  "From": "alice@example.org",
  "To": ["bob@example.org"],
  "Error": "MissingRegion: could not find region configuration"
}
```

## Development

### Build

First, clone the project and then switch into its source directory:

```sh
git clone https://github.com/KamorionLabs/aws-smtp-relay.git
cd aws-smtp-relay
```

**Please note**:

> This project relies on [Go modules](https://github.com/golang/go/wiki/Modules)
> for automatic dependency resolution.

To build the project, run
[Make](<https://en.wikipedia.org/wiki/Make_(software)>) in the repository
directory, which creates the `aws-smtp-relay` binary:

```sh
make
```

### Lint

To lint the source code, first install [staticcheck](https://staticcheck.io/):

```sh
go install honnef.co/go/tools/cmd/staticcheck@latest
```

Then run the following command:

```sh
make lint
```

### Test

All components come with unit tests, which can be executed the following way:

```sh
make test
```

Sending mails can also be tested with the provided [mail shell script](mail.sh):

```sh
echo TEXT | ./mail.sh -p 1025 -f alice@example.org -t bob@example.org
```

**Please note**:

> The provided shell script only supports the `LOGIN` authentication mechanism.

See also
[Testing Amazon SES Email Sending](https://docs.aws.amazon.com/ses/latest/DeveloperGuide/mailbox-simulator.html).

### Install

The binary can also be built and installed in `$GOPATH/bin/` with the following
command:

```sh
make install
```

### Uninstall

The uninstall command removes the binary from `$GOPATH/bin/`:

```sh
make uninstall
```

### Clean

To remove any build artifacts, run the following:

```sh
make clean
```

## Dependencies

- [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto)
- [github.com/emersion/go-smtp](https://github.com/emersion/go-smtp)
- [github.com/aws/aws-sdk-go-v2](https://github.com/aws/aws-sdk-go-v2)

## License

Released under the [MIT license](LICENSE.txt).
