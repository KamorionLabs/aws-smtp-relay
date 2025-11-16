# Deploying AWS SMTP Relay on Amazon EKS

This guide covers deploying AWS SMTP Relay on Amazon EKS using Helm with EKS Pod Identity for secure AWS authentication.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [EKS Pod Identity Setup](#eks-pod-identity-setup)
3. [Helm Deployment](#helm-deployment)
4. [Configuration](#configuration)
5. [Verification](#verification)
6. [Troubleshooting](#troubleshooting)

## Prerequisites

### Required Tools

- `kubectl` (1.19+)
- `helm` (3.0+)
- `aws-cli` (2.0+)
- `eksctl` (optional, for cluster creation)

### AWS Resources

- Amazon EKS cluster (1.24+)
- EKS Pod Identity enabled on the cluster
- IAM permissions to create roles and policies
- SES or Pinpoint configured and verified

### Verify EKS Cluster

```bash
# Check cluster status
aws eks describe-cluster --name YOUR_CLUSTER_NAME --query 'cluster.status'

# Update kubeconfig
aws eks update-kubeconfig --name YOUR_CLUSTER_NAME --region YOUR_REGION

# Verify connection
kubectl get nodes
```

## EKS Pod Identity Setup

EKS Pod Identity provides native AWS IAM integration for Kubernetes pods, delivering credentials through an agent-based approach similar to EC2 instance profiles.

### Key Differences from IRSA

| Feature | EKS Pod Identity | IRSA (IAM Roles for Service Accounts) |
|---------|------------------|----------------------------------------|
| **Setup** | Simpler - cluster-level agent | Per-namespace OIDC configuration |
| **Authentication** | Agent-based credential delivery | Web identity token exchange |
| **Configuration** | Via AWS CLI association | Via ServiceAccount annotation |
| **Annotation** | No Kubernetes annotations needed | Requires `eks.amazonaws.com/role-arn` |
| **Availability** | EKS 1.24+ | EKS 1.13+ |

### Prerequisites

Before configuring Pod Identity, ensure:
1. **EKS cluster version**: 1.24 or later
2. **EKS Pod Identity Agent**: Must be installed as an add-on
3. **AWS CLI**: Version 2.12.0 or later

### Step 0: Install EKS Pod Identity Agent

The agent must be installed once per cluster:

```bash
# Check if agent is already installed
aws eks list-addons --cluster-name YOUR_CLUSTER_NAME | grep eks-pod-identity-agent

# Install the agent as an EKS add-on
aws eks create-addon \
  --cluster-name YOUR_CLUSTER_NAME \
  --addon-name eks-pod-identity-agent \
  --addon-version v1.0.0-eksbuild.1

# Verify installation
kubectl get daemonset eks-pod-identity-agent -n kube-system

# Check agent pods are running on all nodes
kubectl get pods -n kube-system -l app.kubernetes.io/name=eks-pod-identity-agent
```

### Step 1: Create IAM Policy

Choose the appropriate policy based on your relay API:

#### For SES

```bash
# Create policy document
cat > ses-policy.json <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "SendEmailViaSES",
      "Effect": "Allow",
      "Action": [
        "ses:SendEmail",
        "ses:SendRawEmail"
      ],
      "Resource": "*"
    }
  ]
}
EOF

# Create IAM policy
aws iam create-policy \
  --policy-name aws-smtp-relay-ses-policy \
  --policy-document file://ses-policy.json \
  --description "Policy for AWS SMTP Relay to send emails via SES"

# Note the Policy ARN from output
```

#### For Pinpoint

```bash
# Create policy document
cat > pinpoint-policy.json <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "SendEmailViaPinpoint",
      "Effect": "Allow",
      "Action": [
        "mobiletargeting:SendMessages"
      ],
      "Resource": "*"
    }
  ]
}
EOF

# Create IAM policy
aws iam create-policy \
  --policy-name aws-smtp-relay-pinpoint-policy \
  --policy-document file://pinpoint-policy.json \
  --description "Policy for AWS SMTP Relay to send emails via Pinpoint"
```

#### For Cross-Account SES

If using cross-account ARNs, add additional permissions:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "SendEmailViaSES",
      "Effect": "Allow",
      "Action": [
        "ses:SendEmail",
        "ses:SendRawEmail"
      ],
      "Resource": "*"
    },
    {
      "Sid": "AllowCrossAccountSending",
      "Effect": "Allow",
      "Action": [
        "ses:SendEmail",
        "ses:SendRawEmail"
      ],
      "Resource": [
        "arn:aws:ses:*:ACCOUNT_ID:identity/*"
      ]
    }
  ]
}
```

### Step 2: Create IAM Role

Create an IAM role that can be assumed by EKS Pod Identity:

```bash
# Create trust policy for Pod Identity
cat > trust-policy.json <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "pods.eks.amazonaws.com"
      },
      "Action": [
        "sts:AssumeRole",
        "sts:TagSession"
      ]
    }
  ]
}
EOF

# Create IAM role
aws iam create-role \
  --role-name aws-smtp-relay-role \
  --assume-role-policy-document file://trust-policy.json \
  --description "IAM role for AWS SMTP Relay on EKS"

# Note the Role ARN from output
```

### Step 3: Attach Policy to Role

```bash
# Replace with your account ID and policy name
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
POLICY_ARN="arn:aws:iam::${ACCOUNT_ID}:policy/aws-smtp-relay-ses-policy"

aws iam attach-role-policy \
  --role-name aws-smtp-relay-role \
  --policy-arn "${POLICY_ARN}"

# Verify attachment
aws iam list-attached-role-policies --role-name aws-smtp-relay-role
```

### Step 4: Create Kubernetes Namespace and ServiceAccount

Create the namespace first (ServiceAccount will be created by Helm):

```bash
# Create namespace
kubectl create namespace smtp
```

**Note**: Do NOT create the Pod Identity association yet. The ServiceAccount must exist first (created by Helm in next step).

## Helm Deployment

### Step 1: Prepare Values File

Create a custom values file:

```bash
cat > my-values.yaml <<EOF
replicaCount: 2

image:
  repository: ghcr.io/kamorionlabs/aws-smtp-relay
  tag: latest

podIdentity:
  enabled: true

serviceAccount:
  create: true
  name: "aws-smtp-relay"

config:
  addr: ":1025"
  name: "aws-smtp-relay"

  # Authentication
  user: "relay"
  bcryptHash: "YOUR_BCRYPT_HASH"

  # AWS configuration
  relayAPI: "ses"
  awsRegion: "us-east-1"
  setName: "my-configuration-set"

  # Filtering
  allowToDomains: "example.com,example.org"
  maxMessageBytes: 10485760

resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 256Mi

autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 80
EOF
```

### Step 2: Install Helm Chart

```bash
# Install from local chart
helm install aws-smtp-relay ./helm/aws-smtp-relay \
  --namespace smtp \
  --values my-values.yaml

# Or install from repository (if published)
helm repo add aws-smtp-relay https://kamorionlabs.github.io/aws-smtp-relay
helm repo update
helm install aws-smtp-relay aws-smtp-relay/aws-smtp-relay \
  --namespace smtp \
  --values my-values.yaml
```

### Step 3: Verify Deployment

```bash
# Check deployment status
kubectl get deployments -n smtp
kubectl get pods -n smtp
kubectl get svc -n smtp

# Verify ServiceAccount was created
kubectl get sa aws-smtp-relay -n smtp

# View logs
kubectl logs -n smtp -l app.kubernetes.io/name=aws-smtp-relay
```

### Step 4: Create Pod Identity Association

Now that the ServiceAccount exists, create the Pod Identity association:

```bash
# Get your AWS account ID
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)

# Create Pod Identity association
aws eks create-pod-identity-association \
  --cluster-name YOUR_CLUSTER_NAME \
  --namespace smtp \
  --service-account aws-smtp-relay \
  --role-arn arn:aws:iam::${ACCOUNT_ID}:role/aws-smtp-relay-role

# Verify association was created
aws eks list-pod-identity-associations \
  --cluster-name YOUR_CLUSTER_NAME

# Get association details
aws eks describe-pod-identity-association \
  --cluster-name YOUR_CLUSTER_NAME \
  --association-id ASSOCIATION_ID_FROM_PREVIOUS_COMMAND
```

### Step 5: Restart Pods to Apply Pod Identity

Pods must be restarted for EKS to inject the Pod Identity credentials:

```bash
# Restart deployment
kubectl rollout restart deployment/aws-smtp-relay -n smtp

# Watch rollout status
kubectl rollout status deployment/aws-smtp-relay -n smtp

# Verify pods are running with new credentials
kubectl get pods -n smtp

# Check environment variables injected by Pod Identity
kubectl exec -it -n smtp deployment/aws-smtp-relay -- env | grep AWS_CONTAINER
```

You should see environment variables like:
- `AWS_CONTAINER_AUTHORIZATION_TOKEN_FILE`
- `AWS_CONTAINER_CREDENTIALS_FULL_URI`

## Configuration

### Environment Variables

All configuration can be set via Helm values under `config.*`:

| Helm Value | Environment Variable | Description |
|------------|---------------------|-------------|
| `config.addr` | `ADDR` | SMTP server bind address |
| `config.name` | `NAME` | SMTP server hostname |
| `config.user` | `USER` | SMTP username |
| `config.password` | `PASSWORD` | SMTP password (plain) |
| `config.bcryptHash` | `BCRYPT_HASH` | SMTP password (bcrypt) |
| `config.relayAPI` | `RELAY_API` | AWS API (`ses` or `pinpoint`) |
| `config.awsRegion` | `AWS_REGION` | AWS region |
| `config.setName` | `SET_NAME` | SES Configuration Set |
| `config.sourceArn` | `SOURCE_ARN` | Source ARN |
| `config.fromArn` | `FROM_ARN` | From ARN |
| `config.returnPathArn` | `RETURN_PATH_ARN` | Return path ARN |
| `config.ips` | `IPS` | IP whitelist |
| `config.allowFrom` | `ALLOW_FROM` | Sender whitelist regex |
| `config.denyTo` | `DENY_TO` | Recipient blacklist regex |
| `config.allowTo` | `ALLOW_TO` | Recipient whitelist regex |
| `config.allowToDomains` | `ALLOW_TO_DOMAINS` | Allowed domains |
| `config.maxMessageBytes` | `MAX_MESSAGE_BYTES` | Max message size |

### Generating Bcrypt Hash

```bash
# Install htpasswd (if not available)
# macOS: brew install httpd
# Ubuntu: apt-get install apache2-utils

# Generate bcrypt hash
htpasswd -bnBC 10 "" yourpassword | tr -d ':\n' | sed 's/$2y/$2a/'
```

### Using Existing Secrets

To use existing Kubernetes secrets:

```yaml
# In values.yaml
envFrom:
  - secretRef:
      name: aws-smtp-relay-secrets
```

Create the secret:

```bash
kubectl create secret generic aws-smtp-relay-secrets \
  --namespace smtp \
  --from-literal=PASSWORD=yourpassword \
  --from-literal=BCRYPT_HASH=your-bcrypt-hash
```

## Verification

### Test SMTP Connection

From within the cluster:

```bash
kubectl run -it --rm test-smtp --image=alpine --restart=Never --namespace smtp -- sh

# Inside the pod
apk add --no-cache openssl mailx

# Test connection
echo "Subject: Test
From: sender@example.com
To: recipient@example.com

Test message" | nc aws-smtp-relay.smtp.svc.cluster.local 1025
```

### Test with Authentication

```bash
# Create test script
cat > test-smtp.sh <<'EOF'
#!/bin/bash
set -e

SMTP_HOST="aws-smtp-relay.smtp.svc.cluster.local"
SMTP_PORT="1025"
FROM="sender@example.com"
TO="recipient@example.com"

# Send test email
(
  echo "HELO test"
  echo "MAIL FROM:<${FROM}>"
  echo "RCPT TO:<${TO}>"
  echo "DATA"
  echo "Subject: Test from EKS"
  echo "From: ${FROM}"
  echo "To: ${TO}"
  echo ""
  echo "This is a test message from AWS SMTP Relay on EKS."
  echo "."
  echo "QUIT"
) | nc ${SMTP_HOST} ${SMTP_PORT}
EOF

chmod +x test-smtp.sh

# Run inside cluster
kubectl run -it --rm test-smtp --image=alpine --restart=Never --namespace smtp -- sh -c '
  apk add --no-cache netcat-openbsd
  cat > /tmp/test.sh <<EOF
#!/bin/sh
(
  echo "HELO test"
  echo "MAIL FROM:<sender@example.com>"
  echo "RCPT TO:<recipient@example.com>"
  echo "DATA"
  echo "Subject: Test"
  echo ""
  echo "Test message"
  echo "."
  echo "QUIT"
) | nc aws-smtp-relay.smtp.svc.cluster.local 1025
EOF
  chmod +x /tmp/test.sh
  /tmp/test.sh
'
```

### Check AWS Credentials

Verify the pod can access AWS:

```bash
kubectl exec -it -n smtp deployment/aws-smtp-relay -- env | grep AWS

# Should show AWS_REGION and AWS SDK will use Pod Identity
```

### Monitor Logs

```bash
# Follow logs
kubectl logs -f -n smtp -l app.kubernetes.io/name=aws-smtp-relay

# Check for successful sends
kubectl logs -n smtp -l app.kubernetes.io/name=aws-smtp-relay | grep "email sent"

# Check for errors
kubectl logs -n smtp -l app.kubernetes.io/name=aws-smtp-relay | grep -i error
```

## Troubleshooting

### Pod Identity Not Working

**Symptoms**: AWS API calls fail with authentication errors

**Solutions**:

1. Verify Pod Identity association exists:
```bash
aws eks list-pod-identity-associations --cluster-name YOUR_CLUSTER_NAME
```

2. Check ServiceAccount annotation:
```bash
kubectl describe sa aws-smtp-relay -n smtp
# Should show: eks.amazonaws.com/role-arn
```

3. Verify IAM role trust policy:
```bash
aws iam get-role --role-name aws-smtp-relay-role --query 'Role.AssumeRolePolicyDocument'
```

4. Check IAM policy is attached:
```bash
aws iam list-attached-role-policies --role-name aws-smtp-relay-role
```

### Pods Not Starting

**Symptoms**: Pods in `CrashLoopBackOff` or `ImagePullBackOff`

**Solutions**:

1. Check pod events:
```bash
kubectl describe pod -n smtp -l app.kubernetes.io/name=aws-smtp-relay
```

2. Check image availability:
```bash
kubectl get events -n smtp
```

3. Verify image pull secrets (if using private registry):
```bash
kubectl get secrets -n smtp
```

### SES Send Failures

**Symptoms**: SMTP accepts email but SES returns errors

**Solutions**:

1. Verify SES identities are verified:
```bash
aws ses list-identities
aws ses get-identity-verification-attributes --identities your@example.com
```

2. Check SES sending limits:
```bash
aws ses get-send-quota
```

3. Check SES configuration set exists:
```bash
aws ses describe-configuration-set --configuration-set-name YOUR_SET_NAME
```

4. Review CloudWatch Logs for SES events

### Network Connectivity Issues

**Symptoms**: Cannot connect to SMTP service

**Solutions**:

1. Verify service exists:
```bash
kubectl get svc -n smtp aws-smtp-relay
```

2. Check service endpoints:
```bash
kubectl get endpoints -n smtp aws-smtp-relay
```

3. Test from another pod:
```bash
kubectl run -it --rm debug --image=alpine --restart=Never -- sh
nc -zv aws-smtp-relay.smtp.svc.cluster.local 1025
```

4. Check network policies:
```bash
kubectl get networkpolicies -n smtp
```

### High Resource Usage

**Symptoms**: Pods consuming too much CPU/memory

**Solutions**:

1. Check current resource usage:
```bash
kubectl top pods -n smtp
```

2. Adjust resource limits in values:
```yaml
resources:
  limits:
    cpu: 1000m
    memory: 512Mi
  requests:
    cpu: 200m
    memory: 256Mi
```

3. Enable HPA if not already:
```yaml
autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 20
```

### Permission Denied Errors

**Symptoms**: Container fails to start with permission errors

**Solutions**:

1. Check security contexts are correct:
```bash
kubectl get pod -n smtp -o jsonpath='{.items[0].spec.securityContext}'
```

2. Verify Pod Security Standards:
```bash
kubectl label namespace smtp pod-security.kubernetes.io/enforce=baseline
```

## Best Practices

### Security

1. **Use bcrypt hashes** instead of plain passwords
2. **Enable TLS** for SMTP connections
3. **Restrict IP access** using `config.ips`
4. **Use domain filtering** with `config.allowToDomains`
5. **Enable Pod Security Standards** (baseline or restricted)
6. **Use Network Policies** to restrict pod-to-pod communication

### High Availability

1. **Run multiple replicas** (minimum 2)
2. **Enable HPA** for automatic scaling
3. **Use pod anti-affinity** to spread pods across nodes
4. **Set appropriate resource requests/limits**
5. **Configure health probes** (liveness and readiness)

### Monitoring

1. **Enable CloudWatch Container Insights**
2. **Monitor SES metrics** in CloudWatch
3. **Set up alerts** for pod failures and resource usage
4. **Use Prometheus/Grafana** for detailed metrics
5. **Review logs regularly** for errors and patterns

### Cost Optimization

1. **Right-size resource requests**
2. **Use spot instances** for non-critical workloads
3. **Configure appropriate HPA thresholds**
4. **Monitor SES costs** via AWS Cost Explorer
5. **Use SES configuration sets** for detailed usage tracking

## Updating

### Upgrade Helm Release

```bash
# Update values
vim my-values.yaml

# Upgrade release
helm upgrade aws-smtp-relay ./helm/aws-smtp-relay \
  --namespace smtp \
  --values my-values.yaml

# Check rollout status
kubectl rollout status deployment/aws-smtp-relay -n smtp
```

### Rollback

```bash
# View revision history
helm history aws-smtp-relay -n smtp

# Rollback to previous version
helm rollback aws-smtp-relay -n smtp

# Rollback to specific revision
helm rollback aws-smtp-relay 2 -n smtp
```

## Uninstalling

```bash
# Delete Helm release
helm uninstall aws-smtp-relay --namespace smtp

# Delete Pod Identity association
aws eks delete-pod-identity-association \
  --cluster-name YOUR_CLUSTER_NAME \
  --association-id ASSOCIATION_ID

# Delete namespace
kubectl delete namespace smtp

# Optional: Delete IAM resources
aws iam detach-role-policy \
  --role-name aws-smtp-relay-role \
  --policy-arn arn:aws:iam::${ACCOUNT_ID}:policy/aws-smtp-relay-ses-policy

aws iam delete-role --role-name aws-smtp-relay-role
aws iam delete-policy --policy-arn arn:aws:iam::${ACCOUNT_ID}:policy/aws-smtp-relay-ses-policy
```

## Additional Resources

- [EKS Pod Identity Documentation](https://docs.aws.amazon.com/eks/latest/userguide/pod-identities.html)
- [Amazon SES Developer Guide](https://docs.aws.amazon.com/ses/latest/dg/)
- [Amazon Pinpoint Developer Guide](https://docs.aws.amazon.com/pinpoint/latest/developerguide/)
- [Helm Documentation](https://helm.sh/docs/)
- [Kubernetes Documentation](https://kubernetes.io/docs/)

## Support

For issues and questions:
- GitHub Issues: https://github.com/KamorionLabs/aws-smtp-relay/issues
- GitHub Discussions: https://github.com/KamorionLabs/aws-smtp-relay/discussions
