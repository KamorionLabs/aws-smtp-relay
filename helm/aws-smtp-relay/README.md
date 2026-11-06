# aws-smtp-relay Helm chart

Deploys [aws-smtp-relay](https://github.com/KamorionLabs/aws-smtp-relay), an
SMTP server that relays email to Amazon SES or Amazon Pinpoint with the IAM
role of the pod.

- AWS credentials from the service account: EKS Pod Identity or IRSA, no keys
- Prometheus metrics and `/healthz` probes, optional `ServiceMonitor`
- Non-root, read-only root filesystem, all capabilities dropped
- PodDisruptionBudget, pod anti-affinity and optional HorizontalPodAutoscaler

For the IAM role and the Pod Identity association, see the
[EKS deployment guide](../../docs/EKS_DEPLOYMENT.md).

## Install

```sh
helm install aws-smtp-relay oci://ghcr.io/kamorionlabs/charts/aws-smtp-relay \
  --namespace smtp --create-namespace \
  --set config.awsRegion=eu-west-1
```

Each release of the relay publishes the chart with the same version, and that
version as the default image tag. From a clone, install `./helm/aws-smtp-relay`.

Clients in the cluster send to `aws-smtp-relay.smtp.svc:1025`.

Set `image.tag` to run another release of the relay.

## Values

### Relay

| Value                   | Environment variable         | Default            |
| ----------------------- | ---------------------------- | ------------------ |
| `config.port`           | `LISTEN_ADDRESS` (`:<port>`) | `1025`             |
| `config.name`           | `SMTP_SERVICE_NAME`          | `AWS SMTP Relay`   |
| `config.hostname`       | `HOSTNAME`                   | pod name           |
| `config.relayAPI`       | `RELAY_API` (`ses`/`pinpoint`) | `ses`            |
| `config.awsRegion`      | `AWS_REGION`                 | `""`               |
| `config.setName`        | `SES_CONFIGURATION_SET_NAME` | `""`               |
| `config.sourceArn`      | `SES_SOURCE_ARN`             | `""`               |
| `config.fromArn`        | `SES_FROM_ARN`               | `""`               |
| `config.returnPathArn`  | `SES_RETURN_PATH_ARN`        | `""`               |

### Authentication and filters

| Value                   | Environment variable          | Default |
| ----------------------- | ----------------------------- | ------- |
| `config.user`           | `AUTH_USERNAME`               | `""`    |
| `config.bcryptHash`     | `BCRYPT_HASH` (from a secret) | `""`    |
| `config.password`       | `PASSWORD` (from a secret)    | `""`    |
| `config.existingSecret` | secret with keys `bcrypt-hash` and/or `password` | `""` |
| `config.enableLogin`    | `ENABLE_LOGIN`                | `false` |
| `config.ips`            | `ALLOWED_IPS` (exact IPs, comma-separated) | `""` |
| `config.allowFrom`      | `ALLOWED_SENDERS_REGEX`       | `""`    |
| `config.denyTo`         | `DENIED_RECIPIENTS_REGEX`     | `""`    |
| `config.allowTo`        | `ALLOWED_RECIPIENTS_REGEX`    | `""`    |
| `config.allowToDomains` | `ALLOWED_RECIPIENT_DOMAINS`   | `""`    |

With `bcryptHash`, the relay offers `PLAIN` and `LOGIN` (over TLS, or with
`enableLogin`). With `password`, it also offers `CRAM-MD5`. Prefer
`existingSecret` to keep secrets out of the values.

Without `user` or `ips`, any client that can reach the service can send email:
restrict access with a NetworkPolicy.

### TLS

| Value                         | Description                                         | Default |
| ----------------------------- | --------------------------------------------------- | ------- |
| `config.tls.enabled`          | Mount a certificate and set `CERT_FILE`/`KEY_FILE`  | `false` |
| `config.tls.existingSecret`   | Existing `kubernetes.io/tls` secret (cert-manager)  | `""`    |
| `config.tls.cert`, `.key`     | PEM certificate and key, when no existing secret    | `""`    |
| `config.tls.keyPassphrase`    | `TLS_KEY_PASS`                                      | `""`    |
| `config.tls.requireStartTLS`  | `REQUIRE_STARTTLS`                                  | `true`  |
| `config.tls.onlyTLS`          | `REQUIRE_TLS`                                       | `false` |

### Metrics and probes

| Value                               | Description                                  | Default |
| ----------------------------------- | -------------------------------------------- | ------- |
| `metrics.enabled`                   | Serve `/metrics` and `/healthz` (`METRICS_ADDRESS`) | `true` |
| `metrics.port`                      | Metrics port                                 | `9090`  |
| `metrics.serviceMonitor.enabled`    | Create a Prometheus Operator `ServiceMonitor` | `false` |
| `livenessProbe`, `readinessProbe`   | Probe timings; `GET /healthz` with metrics, else TCP on the SMTP port | enabled |

### Kubernetes

| Value                                   | Default |
| --------------------------------------- | ------- |
| `replicaCount`                          | `2`     |
| `image.repository`                      | `ghcr.io/kamorionlabs/aws-smtp-relay` |
| `image.tag`                             | chart `appVersion` |
| `serviceAccount.create`                 | `true`  |
| `serviceAccount.name`                   | release full name |
| `serviceAccount.annotations`            | `{}` (IRSA: `eks.amazonaws.com/role-arn`) |
| `serviceAccount.automountServiceAccountToken` | `false` |
| `service.type`, `service.port`          | `ClusterIP`, `1025` |
| `autoscaling.enabled`                   | `false` |
| `podDisruptionBudget.enabled`           | `true` (`maxUnavailable: 1`) |
| `podAntiAffinityPreset`                 | `soft` (`hard`, or `""`; ignored when `affinity` is set) |
| `env`, `envFrom`                        | `[]`    |
| `resources`                             | requests `100m`/`128Mi`, limit `256Mi` |

See [values.yaml](values.yaml) for all values and
[examples/values-eks-pod-identity.yaml](examples/values-eks-pod-identity.yaml)
for an EKS example.
