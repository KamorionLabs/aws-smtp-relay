# AWS SMTP Relay

[![Docker Image Version](https://img.shields.io/docker/v/kamorion/aws-smtp-relay?sort=semver)](https://hub.docker.com/r/kamorion/aws-smtp-relay)
[![Docker Pulls](https://img.shields.io/docker/pulls/kamorion/aws-smtp-relay)](https://hub.docker.com/r/kamorion/aws-smtp-relay)
[![Docker Image Size](https://img.shields.io/docker/image-size/kamorion/aws-smtp-relay)](https://hub.docker.com/r/kamorion/aws-smtp-relay)

SMTP server to relay emails via **Amazon SES** or **Amazon Pinpoint** using IAM roles.

## 🔧 Why Use This?

Amazon SES and Pinpoint SMTP interfaces require credentials, but using **IAM roles** is more secure. This relay provides an SMTP interface that uses the AWS API with IAM roles instead of SMTP credentials.

## 🚀 Quick Start

```bash
docker run -d \
  -p 1025:1025 \
  -e AWS_REGION=eu-west-1 \
  kamorion/aws-smtp-relay
```

## 📦 Multi-Architecture Support

This image supports:
- `linux/amd64` (x86_64)
- `linux/arm64` (ARM64/aarch64)

## 🔐 IAM Permissions Required

Your container needs IAM permissions to send emails via SES or Pinpoint:

```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Action": [
      "ses:SendRawEmail"
    ],
    "Resource": "*"
  }]
}
```

## 🎯 Common Use Cases

### Docker Compose

```yaml
services:
  smtp-relay:
    image: kamorion/aws-smtp-relay:latest
    ports:
      - "1025:1025"
    environment:
      - AWS_REGION=eu-west-1
```

### With Authentication

```bash
docker run -d \
  -p 1025:1025 \
  -e AWS_REGION=eu-west-1 \
  -e BCRYPT_HASH='$2y$10$...' \
  kamorion/aws-smtp-relay -u username
```

### Cross-Account SES Authorization

```bash
docker run -d \
  -p 1025:1025 \
  -e AWS_REGION=eu-west-1 \
  kamorion/aws-smtp-relay \
  -o arn:aws:ses:region:account-id:identity/example.com
```

## 🔒 Security Options

### Require TLS

```bash
docker run -d \
  -p 1025:1025 \
  -e AWS_REGION=eu-west-1 \
  -v /path/to/certs:/certs:ro \
  kamorion/aws-smtp-relay \
  -c /certs/cert.pem \
  -k /certs/key.pem \
  -s
```

### IP Restriction

```bash
docker run -d \
  -p 1025:1025 \
  -e AWS_REGION=eu-west-1 \
  kamorion/aws-smtp-relay \
  -i 172.17.0.0/16
```

## ⚙️ Configuration Options

| Flag | Description | Default |
|------|-------------|---------|
| `-a` | TCP listen address | `:1025` |
| `-n` | SMTP service name | `AWS SMTP Relay` |
| `-r` | Relay API (ses\|pinpoint) | `ses` |
| `-u` | Authentication username | - |
| `-i` | Allowed client IPs | - |
| `-l` | Allowed sender emails regex | - |
| `-d` | Denied recipient emails regex | - |
| `-o` | Amazon SES SourceArn | - |
| `-f` | Amazon SES FromArn | - |
| `-p` | Amazon SES ReturnPathArn | - |
| `-s` | Require TLS via STARTTLS | `false` |
| `-t` | TLS connections only | `false` |
| `-c` | TLS cert file | - |
| `-k` | TLS key file | - |

## 📚 Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `AWS_REGION` | AWS region for SES/Pinpoint | Yes |
| `BCRYPT_HASH` | Bcrypt password hash for auth | No |
| `PASSWORD` | Plain text password (enables CRAM-MD5) | No |
| `TLS_KEY_PASS` | TLS key file passphrase | No |

## 🔗 Links

- **GitHub**: [KamorionLabs/aws-smtp-relay](https://github.com/KamorionLabs/aws-smtp-relay)
- **GitHub Container Registry**: `ghcr.io/kamorionlabs/aws-smtp-relay`
- **Documentation**: [Full README](https://github.com/KamorionLabs/aws-smtp-relay/blob/main/README.md)
- **Issues**: [Report a bug](https://github.com/KamorionLabs/aws-smtp-relay/issues)

## 🤝 Fork Notice

This is a maintained fork of [blueimp/aws-smtp-relay](https://github.com/blueimp/aws-smtp-relay) with:
- Active maintenance
- Cross-account SES authorization support
- Multi-architecture builds (amd64, arm64)
- Updated dependencies and Go 1.21

Pull requests and contributions welcome!

## 📝 License

Released under the [MIT license](https://github.com/KamorionLabs/aws-smtp-relay/blob/main/LICENSE.txt).
