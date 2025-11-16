# AWS SMTP Relay Helm Chart

A Helm chart for deploying AWS SMTP Relay on Kubernetes with Amazon EKS Pod Identity support.

## Features

- 🚀 Easy deployment to Amazon EKS
- 🔐 Native AWS authentication via EKS Pod Identity
- 📧 Support for AWS SES and Pinpoint
- 🔒 CRAM-MD5 authentication support
- 🛡️ Security-hardened containers
- 📊 Horizontal Pod Autoscaling
- 🔍 Health checks and monitoring
- 🎯 Advanced recipient filtering

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- Amazon EKS cluster (1.24+)
- **EKS Pod Identity Agent** installed on the cluster
- AWS IAM role configured for SES/Pinpoint access
- AWS CLI 2.12.0+ for creating Pod Identity associations

## Installation

### 1. Add the Helm repository (if published)

```bash
helm repo add aws-smtp-relay https://kamorionlabs.github.io/aws-smtp-relay
helm repo update
```

### 2. Install from local chart

```bash
# Clone the repository
git clone https://github.com/KamorionLabs/aws-smtp-relay.git
cd aws-smtp-relay/helm

# Install the chart
helm install aws-smtp-relay ./aws-smtp-relay \
  --namespace smtp \
  --create-namespace \
  --set podIdentity.enabled=true \
  --set config.awsRegion="us-east-1"

# After installation, create the Pod Identity association
aws eks create-pod-identity-association \
  --cluster-name YOUR_CLUSTER_NAME \
  --namespace smtp \
  --service-account aws-smtp-relay \
  --role-arn arn:aws:iam::YOUR_ACCOUNT_ID:role/YOUR_ROLE_NAME

# Restart pods to apply credentials
kubectl rollout restart deployment/aws-smtp-relay -n smtp
```

### 3. Install with custom values

```bash
helm install aws-smtp-relay ./aws-smtp-relay \
  --namespace smtp \
  --create-namespace \
  --values aws-smtp-relay/examples/values-eks-pod-identity.yaml
```

## Configuration

### Required Values

| Parameter | Description | Default |
|-----------|-------------|---------|
| `podIdentity.enabled` | Enable Pod Identity support | `false` |
| `config.awsRegion` | AWS region for SES/Pinpoint | `""` |

**Note**: Pod Identity associations are created via AWS CLI, not Helm values.

### Authentication

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.user` | SMTP username | `""` |
| `config.password` | SMTP password (plain) | `""` |
| `config.bcryptHash` | SMTP password (bcrypt hash) | `""` |

Generate bcrypt hash:
```bash
htpasswd -bnBC 10 "" yourpassword | tr -d ':\n' | sed 's/$2y/$2a/'
```

### AWS Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.relayAPI` | AWS API to use (`ses` or `pinpoint`) | `"ses"` |
| `config.setName` | SES Configuration Set name | `""` |
| `config.sourceArn` | Source ARN for cross-account | `""` |
| `config.fromArn` | From ARN for cross-account | `""` |
| `config.returnPathArn` | Return path ARN for cross-account | `""` |

### Filtering

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.ips` | IP whitelist (comma-separated) | `""` |
| `config.allowFrom` | Sender whitelist regex | `""` |
| `config.denyTo` | Recipient blacklist regex | `""` |
| `config.allowTo` | Recipient whitelist regex | `""` |
| `config.allowToDomains` | Allowed domains (comma-separated) | `""` |
| `config.maxMessageBytes` | Maximum message size | `10485760` |

### Scaling

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of replicas (if autoscaling disabled) | `2` |
| `autoscaling.enabled` | Enable HPA | `false` |
| `autoscaling.minReplicas` | Minimum replicas | `2` |
| `autoscaling.maxReplicas` | Maximum replicas | `10` |
| `autoscaling.targetCPUUtilizationPercentage` | Target CPU % | `80` |

### Resources

| Parameter | Description | Default |
|-----------|-------------|---------|
| `resources.requests.cpu` | CPU request | `100m` |
| `resources.requests.memory` | Memory request | `128Mi` |
| `resources.limits.cpu` | CPU limit | `500m` |
| `resources.limits.memory` | Memory limit | `256Mi` |

## EKS Pod Identity Setup

See **[EKS_DEPLOYMENT.md](../../docs/EKS_DEPLOYMENT.md)** for complete setup instructions.

### Quick Setup

1. **Install EKS Pod Identity Agent** (if not already installed)

```bash
aws eks create-addon \
  --cluster-name YOUR_CLUSTER_NAME \
  --addon-name eks-pod-identity-agent
```

2. **Create IAM Policy**

```bash
aws iam create-policy \
  --policy-name aws-smtp-relay-ses-policy \
  --policy-document file://examples/iam-policy-ses.json
```

3. **Create IAM Role**

```bash
aws iam create-role \
  --role-name aws-smtp-relay-role \
  --assume-role-policy-document '{
    "Version": "2012-10-17",
    "Statement": [{
      "Effect": "Allow",
      "Principal": {
        "Service": "pods.eks.amazonaws.com"
      },
      "Action": ["sts:AssumeRole", "sts:TagSession"]
    }]
  }'
```

4. **Attach Policy to Role**

```bash
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
aws iam attach-role-policy \
  --role-name aws-smtp-relay-role \
  --policy-arn arn:aws:iam::${ACCOUNT_ID}:policy/aws-smtp-relay-ses-policy
```

5. **Install Helm Chart**

```bash
kubectl create namespace smtp

helm install aws-smtp-relay ./aws-smtp-relay \
  --namespace smtp \
  --set podIdentity.enabled=true \
  --set config.awsRegion="us-east-1"
```

6. **Create Pod Identity Association**

```bash
aws eks create-pod-identity-association \
  --cluster-name YOUR_CLUSTER_NAME \
  --namespace smtp \
  --service-account aws-smtp-relay \
  --role-arn arn:aws:iam::${ACCOUNT_ID}:role/aws-smtp-relay-role
```

7. **Restart Pods to Apply Credentials**

```bash
kubectl rollout restart deployment/aws-smtp-relay -n smtp
kubectl rollout status deployment/aws-smtp-relay -n smtp
```

## Testing

Test the SMTP relay from within the cluster:

```bash
kubectl run -it --rm test-smtp --image=alpine --restart=Never -- sh -c '
  apk add --no-cache openssl
  echo "Subject: Test
From: test@example.com
To: recipient@example.com

Test message body" | openssl s_client -connect aws-smtp-relay.smtp.svc.cluster.local:1025 -starttls smtp
'
```

## Monitoring

Check pod status:
```bash
kubectl get pods -n smtp
kubectl logs -n smtp -l app.kubernetes.io/name=aws-smtp-relay
```

Check Pod Identity:
```bash
kubectl describe sa aws-smtp-relay -n smtp
aws eks list-pod-identity-associations --cluster-name YOUR_CLUSTER_NAME
```

## Uninstall

```bash
helm uninstall aws-smtp-relay --namespace smtp
kubectl delete namespace smtp
```

## Troubleshooting

### Pod not starting
- Check IAM role ARN is correct
- Verify Pod Identity association exists
- Check logs: `kubectl logs -n smtp -l app.kubernetes.com/name=aws-smtp-relay`

### Authentication failures with AWS
- Verify IAM policy allows SES/Pinpoint actions
- Check Pod Identity association is active
- Verify AWS region is correct

### SMTP connection issues
- Verify service is running: `kubectl get svc -n smtp`
- Check health probes: `kubectl describe pod -n smtp`
- Test connectivity: `kubectl port-forward -n smtp svc/aws-smtp-relay 1025:1025`

## Support

For issues and questions:
- GitHub Issues: https://github.com/KamorionLabs/aws-smtp-relay/issues
- Documentation: https://github.com/KamorionLabs/aws-smtp-relay

## License

See [LICENSE](../../LICENSE) in the repository root.
